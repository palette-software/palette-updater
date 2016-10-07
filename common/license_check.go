package common

import (
	"encoding/json"
	"fmt"

	insight_server "github.com/palette-software/insight-server/lib"
	log "github.com/palette-software/insight-tester/common/logging"
)

func GetLicenseData(baseFolder string) (*insight_server.LicenseData, error) {

	client, err := NewApiClient(baseFolder)
	if err != nil {
		log.Error("Failed to create Insight API client for acquiring license data! Error: ", err)
		return nil, err
	}
	return getLicenseDataForClient(client)
}

func GetLicenseDataForConfig(config Config) (*insight_server.LicenseData, error) {
	client, err := NewApiClientWithConfig(config)
	if err != nil {
		log.Error("Failed to create Insight API client for acquiring license data! Error: ", err)
		return nil, err
	}
	return getLicenseDataForClient(client)
}

func getLicenseDataForClient(client *ApiClient) (*insight_server.LicenseData, error) {
	resp, err := client.Get("/license")
	if err != nil {
		// The error has already been logged
		return nil, err
	}
	defer resp.Body.Close()

	// Decode the JSON in the response
	licenseData := insight_server.LicenseData{}
	if err := json.NewDecoder(resp.Body).Decode(&licenseData); err != nil {
		return nil, fmt.Errorf("Error while deserializing license data response body: %v -- Error message: %v",
			resp.Body, err)
	}

	return &licenseData, nil
}
