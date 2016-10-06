package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	insight "github.com/palette-software/insight-server/lib"
	log "github.com/palette-software/insight-tester/common/logging"
	"github.com/palette-software/palette-updater/common"
	servdis "github.com/palette-software/palette-updater/services-discovery"
)

func getLatestVersion(product, updateServerAddress string) (insight.UpdateVersion, error) {
	log.Debugf("Getting latest %s version...", product)

	version := insight.UpdateVersion{}
	endpoint := fmt.Sprintf("%s/updates/latest-version?product=%s", updateServerAddress, product)
	resp, err := http.Get(endpoint)
	if err != nil {
		log.Errorf("Error during querying latest %s version from %s: %v", product, endpoint, err)
		return version, err
	}
	log.Debugf("Latest %s version response: %s", product, resp)
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err = fmt.Errorf("Getting latest %s version failed from %s! Server response: %s", product, endpoint, resp)
		log.Error(err)
		return version, err
	}

	// Decode the JSON in the response
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return version, fmt.Errorf("Error while deserializing version response body. Error message: %v", err)
	}

	log.Info("Latest available version: ", version.String())
	return version, nil
}

func getCurrentVersion(product string) (currentVersion insight.Version, err error) {
	var svcToLookUp string

	switch product {
	case "agent":
		svcToLookUp = common.AgentSvcName
	default:
		err = fmt.Errorf("Unexpected product! Failed to map %s to service name!", product)
		log.Error(err)
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
			log.Error(err)
		}
	}()

	// Convert string version into Version
	versionNumbers := strings.Split(versionStr, ".")
	currentVersion.Major, err = strconv.Atoi(versionNumbers[0])
	if err != nil {
		log.Errorf("Failed to retrieve major version for %s! Error message: %s", product, err)
		return insight.Version{0, 0, 0}, err
	}

	currentVersion.Minor, err = strconv.Atoi(versionNumbers[1])
	if err != nil {
		log.Errorf("Failed to retrieve minor version for %s! Error message: %s", product, err)
		return insight.Version{0, 0, 0}, err
	}

	currentVersion.Patch, err = strconv.Atoi(versionNumbers[2])
	if err != nil {
		log.Errorf("Failed to retrieve patch version for %s! Error message: %s", product, err)
		return insight.Version{0, 0, 0}, err
	}

	log.Infof("Currently installed %s version: %s", product, currentVersion.String())
	return currentVersion, err
}

// Downloads the specified version and returns the name of the file or an error if there was any
func downloadVersion(updateServerAddress, product string, version insight.UpdateVersion) (string, error) {
	versionString := version.String()
	log.Infof("Downloading %s version: %s", product, versionString)
	fileName := fmt.Sprintf("%s-%s", product, versionString)
	endpoint := fmt.Sprintf("%s/updates/products/%s/%s/%s", updateServerAddress, product, versionString, fileName)

	// Download
	resp, err := http.Get(endpoint)
	if err != nil {
		log.Errorf("Failed to download %s version: %s", product, versionString)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err = fmt.Errorf("Getting %s version: %s failed! Server response: %s", product, versionString, resp)
		log.Error(err)
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Failed to read contents of downloaded file: %s. Error message: %s", fileName, err)
	}

	// Save the update into the updates folder
	err = os.Mkdir(updatesFolder, 0777)
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			log.Warning("Failed to create updates folder. Error message: ", err)
		}
	}

	// Check the MD5 hash of the downloaded file. If it is not right, retry the download in the next update round.
	savedFileHash := md5.Sum(body)
	latestHash := fmt.Sprintf("%32x", savedFileHash)
	log.Infof("MD5 hash of %s: %s", fileName, latestHash)

	if latestHash != version.Md5 {
		err = fmt.Errorf("MD5 hash mismatch for file: %s! Expected hash is %s, but calculated is %s.",
			fileName, version.Md5, latestHash)
		log.Error(err)
		return "", err
	}

	filePath := filepath.Join(updatesFolder, fileName)

	err = ioutil.WriteFile(fileName, body, 0777)
	if err != nil {
		log.Errorf("Failed to save file: %s! Error message: %s", filePath, err)
		return "", err
	}

	// Successfully downloaded the file
	log.Infof("Saved update file: %s", filePath)
	return filePath, nil
}

func checkForUpdates(product, insightServerAddress string) {
	// Check the latest version available on the server
	latestVersion, err := getLatestVersion(product, insightServerAddress)
	if err != nil {
		log.Errorf("Failed to retrieve latest %s version. Error message: %s", product, err)
		return
	}

	// Obtain the currently installed version
	currentVersion, err := getCurrentVersion(product)
	if err != nil {
		// Errors are logged inside the function
		return
	}

	// Perform the update, if there is a newer version
	if insight.IsNewerVersion(latestVersion.Version, currentVersion) {
		log.Infof("Found newer %s version (%s) on server. Current version is %s",
			product, latestVersion.String(), currentVersion.String())

		// Download the latest version
		updateFilePath, err := downloadVersion(insightServerAddress, product, latestVersion)
		if err != nil {
			return
		}

		err = performCommand("update", updateFilePath)
		if err != nil {
			log.Errorf("Failed to perform the %s update: %s", product, err)
		}
	} else {
		log.Debugf("Current version is %s. Latest available version: %s is not newer. No need to update.",
			currentVersion.String(), latestVersion.Version.String())
	}
}
