// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	insight "github.com/palette-software/insight-server"
	log "github.com/palette-software/insight-tester/common/logging"

	"crypto/md5"
	"github.com/kardianos/osext"
	"gopkg.in/yaml.v2"
	"time"
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

func getCurrentVersion(product string) (insight.Version, error) {
	// FIXME: Find a way to determine the currently installed version of the given product
	return insight.Version{1, 3, 2}, nil
}

func performUpdate() error {
	// FIXME: Implement update process
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

// Download the specified version
func downloadVersion(updateServerAddress, product string, version insight.UpdateVersion) error {
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
			return err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error.Printf("Failed to read contents of downloaded file: %s. Error message: %s", fileName, err)
		}

		err = ioutil.WriteFile(fileName, body, 777)
		if err != nil {
			log.Error.Printf("Failed to save file: %s! Error message: %s", fileName, err)
			return err
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
	return nil
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
		log.Error.Printf("Failed to retrieve current %s version.", product)
		return
	}

	// Perform the update, if there is a newer version
	if latestVersion.String() > currentVersion.String() {
		log.Info.Printf("Found newer %s version on server.", product)

		// Download the latest version
		err = downloadVersion(updateServerAddress, product, latestVersion)
		if err != nil {
			return
		}

		err = performUpdate()
		if err != nil {
			log.Error.Printf("Failed to perform the %s update: %s", product, err)
		}
	}
}
