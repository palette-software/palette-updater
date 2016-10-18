package common

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/palette-software/insight-server/lib"
	log "github.com/palette-software/insight-tester/common/logging"
)

const InsightApiVersion = "v1"

type ApiClient struct {
	httpClient *http.Client
	config     Config
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
	// This is a copy of http.DefaultTransport, but certificate check is disabled.
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,

		// Beware! Certificate check is diabled, because on-premise Insight Server
		// names are not like *.palette-software.net
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	wsConfig := config.Webservice
	if wsConfig.UseProxy {
		if len(wsConfig.ProxyAddress) == 0 {
			err := fmt.Errorf("Missing proxy address from config file, but UseProxy is set!")
			log.Error(err)
			return nil, err
		}
		proxyUrl, err := url.Parse(wsConfig.ProxyAddress)
		if err != nil {
			log.Errorf("Could not parse proxy settings: %s from %s. Error message: %s",
				wsConfig.ProxyAddress, insight_server.AgentConfigFileName, err)
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxyUrl)
	}

	innerClient := &http.Client{
		// Timeout can be really important, because the default is
		// to wait forever, which can make our application to hang
		Timeout:   time.Second * 30,
		Transport: transport,
	}

	return &ApiClient{
		httpClient: innerClient,
		config:     config,
	}, nil
}

func (c *ApiClient) Get(endpoint string) (*http.Response, error) {
	url := c.makeApiUrl(endpoint)
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
		dump := dumpResponse(resp)
		err = fmt.Errorf("API client's GET %s failed! Server response: %v",
			url, dump)
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
	err = os.MkdirAll(filepath.Dir(destinationPath), 777)
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
	url := c.makeApiUrl(endpoint)
	req, err := newfileUploadRequest(url, insight_server.UploadFileParam, sourcePath)
	if err != nil {
		log.Errorf("Failed to upload file: '%s' Error: %v", sourcePath, err)
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		dump := dumpRequest(req)
		err = fmt.Errorf("Client do request failed! Error message: %v\n\tRequest: %v", err, dump)
		log.Error(err)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		dump := dumpResponse(resp)
		err = fmt.Errorf("Upload failed for file: %s. Server response: %v", sourcePath, dump)
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

func (c *ApiClient) makeApiUrl(endpoint string) string {
	url := c.config.Webservice.Endpoint
	if !strings.HasPrefix(endpoint, "/api/") {
		url = fmt.Sprint(url, "/api/", InsightApiVersion)
	}
	return fmt.Sprint(url, endpoint)
}

func dumpResponse(resp *http.Response) string {
	dump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return fmt.Sprintf("Response dump failed! Error: %v\n\tRaw response: %v", err, resp)
	}
	return string(dump)
}

func dumpRequest(req *http.Request) string {
	dump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		return fmt.Sprintf("Request dump failed! Error: %v\n\tRaw request: %v", err, req)
	}
	return string(dump)
}
