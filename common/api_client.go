package common

import (
	"net/http"
	"time"
	"fmt"

	log "github.com/palette-software/insight-tester/common/logging"
)

const InsightApiVersion = "v1"

type ApiClient struct {
	httpClient *http.Client
	config 		Config
	baseUrl		string
}

func NewApiClient(baseFolder string) (*ApiClient, error) {
	innerClient := &http.Client{
		// Timeout can be really important, because the default is
		// to wait forever, which can make our application to hang
		Timeout: time.Second * 30,
	}

	config, err := ParseConfig(baseFolder)
	if err != nil {
		log.Error("Failed to parse config file! Error: ", err)
		return nil, err
	}

	insightServerAddress, err := config.Webservice.GetPreparedEndpoint()
	if err != nil {
		return nil, fmt.Errorf("Failed to get webservice endpoint! Error: %v", err)
	}

	apiClient := &ApiClient{
		httpClient: innerClient,
		config: config,
		baseUrl: fmt.Sprintf("%s/api/%s", insightServerAddress, InsightApiVersion),
	}
	return apiClient, nil
}

func (c *ApiClient) Get(endpoint string) (*http.Response, error) {
	url := fmt.Sprint(c.baseUrl, endpoint)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		err = fmt.Errorf("Failed to create GET request for %s Error: %v", url, err)
		log.Error(err)
		return nil, err
	}

	// Automatically add the token based authorization header
	req.Header.Add("Authorization", fmt.Sprintf("Token %s", c.config.LicenseKey))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		err := fmt.Errorf("Failed to GET response from %s! Error: %v", url, err)
		log.Error(err)
		return nil, err
	}

	if resp.StatusCode != 200 {
		err = fmt.Errorf("GET %s returned status code: %d -- Server response: %v -- Error: %v",
			url, resp.StatusCode, resp, err)
		log.Error(err)
		// Make sure that the response gets closed in this case too
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}
