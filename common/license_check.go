package common

import (
	"encoding/json"
	"fmt"

	insight_server "github.com/palette-software/insight-server/lib"
	log "github.com/palette-software/insight-tester/common/logging"
)

func GetLicenseData(baseFolder string) (insight_server.LicenseData, error) {
	licenseData := insight_server.LicenseData{}

	client, err := NewApiClient(baseFolder)
	if err != nil {
		log.Error("Failed to create Insight API client for acquiring license data! Error: ", err)
	}
	resp, err := client.Get("/license")
	if err != nil {
		// The error has already been logged
		return licenseData, err
	}
	defer resp.Body.Close()

	// Decode the JSON in the response
	if err := json.NewDecoder(resp.Body).Decode(&licenseData); err != nil {
		return licenseData, fmt.Errorf("Error while deserializing license data response body: %v -- Error message: %v",
			resp.Body, err)
	}

	return licenseData, nil
}
