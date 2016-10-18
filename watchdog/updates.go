package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	insight "github.com/palette-software/insight-server/lib"
	log "github.com/palette-software/insight-tester/common/logging"
	"github.com/palette-software/palette-updater/common"
	servdis "github.com/palette-software/palette-updater/services-discovery"
)

func getLatestVersion(client *common.ApiClient) (insight.UpdateVersion, error) {
	log.Debugf("Getting latest agent version...")

	version := insight.UpdateVersion{}
	resp, err := client.Get("/agent/version")
	if err != nil {
		return version, err
	}

	// Decode the JSON in the response
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return version, fmt.Errorf("Error while deserializing version response body. Error message: %v", err)
	}

	log.Info("Latest available version on Insight Server: ", version)
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

	log.Infof("Currently installed %s version: %s", product, currentVersion)
	return currentVersion, err
}

func checkForUpdates() {
	client, err := common.NewApiClient(baseFolder)
	if err != nil {
		log.Error("Check agent update failed! Unable to create API client. Error: ", err)
		return
	}
	// Check the latest version available on the server
	latestUpdate, err := getLatestVersion(client)
	if err != nil {
		log.Error("Failed to retrieve latest agent version. Error message: ", err)
		return
	}
	latestVersion := latestUpdate.Version

	// Obtain the currently installed version
	currentVersion, err := getCurrentVersion("agent")
	if err != nil {
		// Errors are logged inside the function
		return
	}

	// Perform the update, if there is a newer version
	if insight.IsNewerVersion(latestVersion, currentVersion) {
		log.Infof("Found newer agent version (%s) on server. Current version is %s",
			latestVersion, currentVersion)

		// Download the latest version
		updateFileName := fmt.Sprintf("agent-%s", latestVersion)
		updateFilePath := filepath.Join(updatesFolder, updateFileName)
		log.Info("Downloading agent version: ", latestVersion)
		err = client.DownloadFile(latestUpdate.Url, updateFilePath)
		if err != nil {
			log.Errorf("Failed to donwload latest version (%s)! Error: %v", latestVersion, err)
			return
		}
		log.Infof("Saved update file: %s", updateFilePath)

		// Check the MD5 hash of the downloaded file. If it is not right, retry the download in the next update round.
		updateFileBytes, err := ioutil.ReadFile(updateFilePath)
		if err != nil {
			log.Errorf("Failed to read the contents of the newly donwloaded agent update file: %s! Error: %v",
				updateFileName, err)
		}
		downloadedFileHash := md5.Sum(updateFileBytes)
		downloadedHash := fmt.Sprintf("%32x", downloadedFileHash)
		log.Infof("MD5 hash of %s: %s", updateFileName, downloadedHash)

		if downloadedHash != latestUpdate.Md5 {
			err = fmt.Errorf("MD5 hash mismatch for file: %s! Expected hash is %s, but calculated is %s.",
				updateFileName, latestUpdate.Md5, downloadedHash)
			log.Error(err)
			// The downloaded file is corrupted, so delete it
			os.Remove(updateFilePath)
			return
		}

		err = performCommand("update", updateFilePath)
		if err != nil {
			log.Errorf("Failed to perform the agent update: %s", err)
			return
		}
		return
	}

	log.Infof("Current version is %s. Latest available version: %s is not newer. No need to update.",
		currentVersion, latestVersion)
}
