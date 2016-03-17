package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	insight "github.com/palette-software/insight-server"
	log "github.com/palette-software/insight-tester/common/logging"
	servdis "github.com/palette-software/palette-updater/services-discovery"

	"crypto/md5"
	gocp "github.com/cleversoap/go-cp"
	"gopkg.in/yaml.v2"
	"path/filepath"
)

func getLatestVersion(product, updateServerAddress string) (insight.UpdateVersion, error) {
	log.Debug.Printf("Getting latest %s version...", product)

	version := insight.UpdateVersion{}
	endpoint := fmt.Sprintf("%s/updates/latest-version?product=%s", updateServerAddress, product)
	resp, err := http.Get(endpoint)
	if err != nil {
		log.Error.Printf("Error during querying latest %s version: ", product, err)
		return version, err
	}
	log.Debug.Printf("Latest %s version response: %s", product, resp)
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err = fmt.Errorf("Getting latest %s version failed! Server response: %s", product, resp)
		log.Error.Println(err)
		return version, err
	}

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

func performUpdate(updateFilePath string) (err error) {
	tempUpdaterFileName := filepath.Join(baseFolder, "updater_in_action.exe")
	err = gocp.Copy(filepath.Join(baseFolder ,"updater.exe"), tempUpdaterFileName)
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

	log.Info.Printf("Performing update: %s", updateFilePath)
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
	configPath := filepath.Join(baseFolder, "Config", "Config.yml")

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
	filePath := fmt.Sprintf("%s-%s", product, versionString)
	endpoint := fmt.Sprintf("%s/updates/products/%s/%s/%s", updateServerAddress, product, versionString, filePath)

	// Download
	resp, err := http.Get(endpoint)
	if err != nil {
		log.Error.Println("Failed to download %s version: %s", product, versionString)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err = fmt.Errorf("Getting %s version: %s failed! Server response: %s", product, versionString, resp)
		log.Error.Println(err)
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error.Printf("Failed to read contents of downloaded file: %s. Error message: %s", filePath, err)
	}

	// Save the update into the updates folder
	err = os.Mkdir(updatesFolder, 777)
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			log.Warning.Println("Failed to create updates folder. Error message: ", err)
		}
	}

	// Check the MD5 hash of the downloaded file. If it is not right, retry the download in the next update round.
	savedFileHash := md5.Sum(body)
	latestHash := fmt.Sprintf("%32x", savedFileHash)
	log.Info.Printf("MD5 hash of %s: %s", filePath, latestHash)

	if latestHash != version.Md5 {
		err = fmt.Errorf("MD5 hash mismatch for file: %s! Expected hash is %s, but calculated is %s.",
			filePath, version.Md5, latestHash)
		log.Error.Println(err)
		return "", err
	}

	filePath = filepath.Join(updatesFolder, filePath)

	err = ioutil.WriteFile(filePath, body, 777)
	if err != nil {
		log.Error.Printf("Failed to save file: %s! Error message: %s", filePath, err)
		return "", err
	}

	// Successfully downloaded the file
	log.Info.Printf("Saved update file: %s", filePath)
	return filePath, nil
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
	if insight.IsNewerVersion(&latestVersion.Version, &currentVersion) {
		log.Info.Printf("Found newer %s version (%s) on server. Current version is %s",
			product, latestVersion.String(), currentVersion.String())

		// Download the latest version
		updateFilePath, err := downloadVersion(updateServerAddress, product, latestVersion)
		if err != nil {
			return
		}

		err = performUpdate(updateFilePath)
		if err != nil {
			log.Error.Printf("Failed to perform the %s update: %s", product, err)
		}
	} else {
		log.Debug.Printf("Current version is %s. Latest available version: %s is not newer. No need to update.",
			currentVersion.String(), latestVersion.Version.String())
	}
}
