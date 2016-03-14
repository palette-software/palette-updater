package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"os/exec"
	"io/ioutil"

	insight "github.com/palette-software/insight-server"
	log "github.com/palette-software/insight-tester/common/logging"
	servdis "github.com/palette-software/palette-updater/services-discovery"

	"crypto/md5"
	"github.com/kardianos/osext"
	"gopkg.in/yaml.v2"
	gocp "github.com/cleversoap/go-cp"
)

func getLatestVersion(product, updateServerAddress string) (insight.UpdateVersion, error) {
	log.Debug.Printf("Getting latest %s version...", product)

	version := insight.UpdateVersion{}
	endpoint := fmt.Sprintf("%s/updates/latest-version?product=%s", updateServerAddress, product)
	resp, err := http.Get(endpoint)
	if err != nil {
		log.Error.Println("Error during querying latest agent version: ", err)
		return version, err
	}
	log.Debug.Printf("Latest %s version response: %s", product, resp)
	defer resp.Body.Close()

	// Decode the JSON in the response
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return version, fmt.Errorf("Error while deserializing version response body. Error message: %v", err)
	}

	log.Info.Println("Decoded version: ", version.String())
	return version, nil
}

func getCurrentVersion(product string) (currentVersion insight.Version, err error) {
	var svcToLookUp string

	switch product {
	case "agent":
		svcToLookUp = "PaletteInsightAgent"
	default:
		err = fmt.Errorf("Unexpected product! Failed to map %s to service name!", product)
		log.Error.Println(err)
		return insight.Version{0, 0, 0}, err
	}

	versionStr, err := servdis.GetServiceVersion(svcToLookUp)
	if err != nil {
		return insight.Version{0, 0, 0}, err
	}

	// Handle panics during parsing the version string
	defer func() {
		if r := recover(); r != nil {
			// Set the return values
			currentVersion = insight.Version{0, 0, 0}
			err = fmt.Errorf("Panic recovered while getting current version of %s. Recovered value: %s", product, r)
			log.Error.Println(err)
		}
	}()

	// Convert string version into Version
	versionNumbers := strings.Split(versionStr, ".")
	currentVersion.Major, err = strconv.Atoi(versionNumbers[0])
	if err != nil {
		log.Error.Printf("Failed to retrieve major version for %s! Error message: %s", product, err)
		return insight.Version{0, 0, 0}, err
	}

	currentVersion.Minor, err = strconv.Atoi(versionNumbers[1])
	if err != nil {
		log.Error.Printf("Failed to retrieve minor version for %s! Error message: %s", product, err)
		return insight.Version{0, 0, 0}, err
	}

	currentVersion.Patch, err = strconv.Atoi(versionNumbers[2])
	if err != nil {
		log.Error.Printf("Failed to retrieve patch version for %s! Error message: %s", product, err)
		return insight.Version{0, 0, 0}, err
	}

	log.Info.Printf("Obtained version for %s: %s", product, currentVersion.String())
	return currentVersion, err
}

func performUpdate(updateFilePath string) (err error){
	tempUpdaterFileName := "updater_in_action.exe"
	err = gocp.Copy("updater.exe", tempUpdaterFileName)
	if err != nil {
		log.Error.Println("Failed to make copy of updater.exe! Error message: ", err)
		return err
	}
	defer func() {
		err = os.Remove(tempUpdaterFileName)
		if err != nil {
			log.Error.Printf("Failed to delete %s! Error message: %s", tempUpdaterFileName, err)
		}
	}()

	cmd := exec.Command(tempUpdaterFileName, updateFilePath)
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Error.Printf("Failed to execute %s! Error message: %s", tempUpdaterFileName, err)
	}

	log.Info.Printf("Successfully performed update: %s", updateFilePath)
	return nil
}

type Webservice struct {
	Endpoint string `yaml:"Endpoint"`
}

type Config struct {
	Webservice Webservice `yaml:"Webservice"`
}

func obtainUpdateServerAddress() (string, error) {
	configFilePath, err := findAgentConfigFile()
	if err != nil {
		return "", err
	}

	var config Config

	// Open agent's .yml config file
	input, err := os.Open(configFilePath)
	if err != nil {
		log.Error.Println("Error opening file: ", err)
		return "", err
	}
	defer input.Close()
	b, err := ioutil.ReadAll(input)
	if err != nil {
		log.Error.Println("Error reading file: ", err)
		return "", err
	}

	// Parse the .yml config file
	err = yaml.Unmarshal(b, &config)
	if err != nil {
		log.Error.Println("Error parsing xml", err)
		return "", err
	}
	return config.Webservice.Endpoint, nil
}

// FIXME: locating the config file is not generic! This means this way is not going to be okay if we wanted to use this service as an auto-updater for the insight-server
// NOTE: This only works as long as the watchdog service runs from the very same folder as the agent.
// But they are supposed to be in the same folder by design.
func findAgentConfigFile() (string, error) {
	folderPath, err := osext.ExecutableFolder()
	if err != nil {
		log.Fatal("Failed to get the execution folder: ", err)
		return "", err
	}

	log.Debug.Println("Execution folder: ", folderPath)

	configPath := folderPath + "/Config/Config.yml"

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Error.Println("Agent config file does not exist! Error message: ", err)
		return "", err
	}

	// Successfully located agent config file
	return configPath, nil
}

// Downloads the specified version and returns the name of the file or an error if there was any
func downloadVersion(updateServerAddress, product string, version insight.UpdateVersion) (string, error) {
	versionString := version.String()
	log.Info.Printf("Downloading %s version: %s", product, versionString)
	fileName := fmt.Sprintf("%s-%s", product, versionString)
	endpoint := fmt.Sprintf("%s/updates/products/%s/%s/%s", updateServerAddress, product, versionString, fileName)

	// FIXME: This is not platform independent!
	fileName = fileName + ".msi"

	var attemptCounter uint32 = 0

	// Except for the first download attempt, wait for a while before the next download attempt.
	// But not more than 30 minutes.
	const maxWaitSeconds uint32 = 30 * 60
	// The wait duration is increased by 10 seconds on each attempt.
	const waitIncreaseUnit uint32 = 10

	for {
		waitSeconds := attemptCounter * waitIncreaseUnit
		if waitSeconds > maxWaitSeconds {
			waitSeconds = maxWaitSeconds
		}
		time.Sleep(time.Duration(waitSeconds) * time.Second)

		attemptCounter++
		if attemptCounter > 1 {
			log.Info.Printf("Downloading %s version: %s (%d. attempt)", product, versionString, attemptCounter)
		}

		// Download
		resp, err := http.Get(endpoint)
		if err != nil {
			log.Error.Println("Failed to download %s version: %s", product, versionString)
			return "", err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error.Printf("Failed to read contents of downloaded file: %s. Error message: %s", fileName, err)
		}

		err = ioutil.WriteFile(fileName, body, 777)
		if err != nil {
			log.Error.Printf("Failed to save file: %s! Error message: %s", fileName, err)
			return "", err
		}

		// Check the MD5 hash of the downloaded file. If it is not right, retry the download.
		savedFileHash := md5.Sum(body)
		latestHash := fmt.Sprintf("%32x", savedFileHash)
		log.Info.Printf("MD5 hash of %s: %s", fileName, latestHash)

		if latestHash != version.Md5 {
			log.Warning.Printf("MD5 hash mismatch for file: %s! Expected hash is %s, but calculated is %s. Retrying file download.",
				fileName, version.Md5, latestHash)
			continue
		}

		// Successfully downloaded the file
		break
	}

	log.Info.Printf("Saved update file: %s", fileName)
	return fileName, nil
}

func checkForUpdates(product string) {
	// Get the server address which stores the update files
	updateServerAddress, err := obtainUpdateServerAddress()
	if err != nil {
		log.Error.Println("Failed to obtain update server address! Error message: ", err)
		return
	}

	// Check the latest version available on the server
	latestVersion, err := getLatestVersion(product, updateServerAddress)
	if err != nil {
		log.Error.Printf("Failed to retrieve latest %s version. Error message: %s", product, err)
		return
	}

	// Obtain the currently installed version
	currentVersion, err := getCurrentVersion(product)
	if err != nil {
		// Errors are logged inside the function
		return
	}

	// Perform the update, if there is a newer version
	if latestVersion.String() > currentVersion.String() {
		log.Info.Printf("Found newer %s version on server.", product)

		// Download the latest version
		updateFileName, err := downloadVersion(updateServerAddress, product, latestVersion)
		if err != nil {
			return
		}

		folderPath, err := osext.ExecutableFolder()
		if err != nil {
			log.Fatal("Failed to get the execution folder: ", err)
			return
		}

		updateFilePath := folderPath + "\\" + updateFileName
		err = performUpdate(updateFilePath)
		if err != nil {
			log.Error.Printf("Failed to perform the %s update: %s", product, err)
		}
	}
}
