package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	insight_server "github.com/palette-software/insight-server/lib"
	log "github.com/palette-software/insight-tester/common/logging"
	"time"
)

func GetLicenseData(baseFolder string) (insight_server.LicenseData, error) {
	licenseData := insight_server.LicenseData{}

	config, err := ParseConfig(baseFolder)
	if err != nil {
		log.Error("Failed to parse config file! Error: ", err)
		return licenseData, err
	}

	insightServerAddress, err := config.Webservice.GetPreparedEndpoint()
	if err != nil {
		log.Errorf("Failed to get webservice endpoint for checking license key: '%s'", config.LicenseKey)
		return licenseData, err
	}
	endpoint := fmt.Sprintf("%s/api/%s/license", insightServerAddress, InsightApiVersion)

	client := &http.Client{Timeout: time.Second * 10}
	req, err := http.NewRequest("GET", endpoint, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Token %s", config.LicenseKey))

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("Failed to query license data for license key: '%s' Error: %v", config.LicenseKey, err)
		return licenseData, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err = fmt.Errorf("Getting license data for license key: '%s' failed from %s! Server response: %s",
			config.LicenseKey, endpoint, resp)
		log.Error(err)
		return licenseData, err
	}

	// Decode the JSON in the response
	if err := json.NewDecoder(resp.Body).Decode(&licenseData); err != nil {
		return licenseData, fmt.Errorf("Error while deserializing license data response body. Error message: %v", err)
	}

	return licenseData, nil
}
