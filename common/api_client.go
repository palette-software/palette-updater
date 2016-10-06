package common

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	log "github.com/palette-software/insight-tester/common/logging"
	"github.com/palette-software/insight-server/lib"
)

const InsightApiVersion = "v1"

type ApiClient struct {
	httpClient *http.Client
	config     Config
	baseUrl    string
}

func NewApiClient(baseFolder string) (*ApiClient, error) {
	config, err := ParseAgentConfig(baseFolder)
	if err != nil {
		log.Error("Failed to parse config file! Error: ", err)
		return nil, err
	}

	return NewApiClientWithConfig(config)
}

func NewApiClientWithConfig(config Config) (*ApiClient, error) {
	innerClient := &http.Client{
		// Timeout can be really important, because the default is
		// to wait forever, which can make our application to hang
		Timeout: time.Second * 30,
	}

	insightServerAddress, err := config.Webservice.GetPreparedEndpoint()
	if err != nil {
		return nil, fmt.Errorf("Failed to get webservice endpoint! Error: %v", err)
	}

	apiClient := &ApiClient{
		httpClient: innerClient,
		config:     config,
		baseUrl:    fmt.Sprintf("%s/api/%s", insightServerAddress, InsightApiVersion),
	}
	return apiClient, nil
}

func (c *ApiClient) Get(endpoint string) (*http.Response, error) {
	url := fmt.Sprint(c.baseUrl, endpoint)
	req, err := http.NewRequest(http.MethodGet, url, nil)
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

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("GET %s returned status code: %d -- Server response: %v -- Error: %v",
			url, resp.StatusCode, resp, err)
		log.Error(err)
		// Make sure that the response gets closed in this case too
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

func (c *ApiClient) DownloadFile(endpoint, destinationPath string) error {
	resp, err := c.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Failed to read response contents of URL: %s. Error message: %s", endpoint, err)
		return err
	}

	// Save the update into the updates folder
	err = os.MkdirAll(path.Dir(destinationPath), 777)
	if err != nil {
		log.Errorf("Failed to create folders for path: '%s' Error: %v", destinationPath, err)
		return err
	}

	err = ioutil.WriteFile(destinationPath, body, 777)
	if err != nil {
		log.Errorf("Failed to save file: %s! Error message: %s", destinationPath, err)
		return err
	}
	return nil
}

func (c *ApiClient) UploadFile(endpoint, sourcePath string) error {
	url := fmt.Sprint(c.baseUrl, endpoint)
	req, err := newfileUploadRequest(url, insight_server.UploadFileParam, sourcePath)
	if err != nil {
		log.Errorf("Failed to upload file: '%s' Error: %v", sourcePath, err)
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("Client do request failed! Request: %v. Error message: %v", req, err)
		log.Error(err)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("Upload failed for file: %s. Server response: %v", sourcePath, resp)
		log.Error(err)
		return err
	}
	return nil
}

// Creates a new file upload http request with multipart file
func newfileUploadRequest(uri string, paramName, path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPut, uri, body)
	if err != nil {
		return nil, fmt.Errorf("Failed to create new request! Error message: %v", err)
	}

	req.Header.Add("Content-Type", writer.FormDataContentType())

	return req, nil
}
