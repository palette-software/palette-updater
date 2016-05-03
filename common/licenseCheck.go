package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	insight_server "github.com/palette-software/insight-server/lib"

	"github.com/kardianos/osext"
)

func GetOwner() (string, error) {
	execFolder, err := osext.ExecutableFolder()
	if err != nil {
		return "", fmt.Errorf("Failed to get executable folder for Splunk target: %v", err)
	}

	// Check for license
	files, err := ioutil.ReadDir(execFolder)
	if err != nil {
		return "", fmt.Errorf("Failed to read exec dir for license files: %v", err)
	}

	insightServerAddress, err := ObtainInsightServerAddress(execFolder)
	if err != nil {
		return "", fmt.Errorf("Failed to get Insight Server address: %v", err)
	}

	ownerName := ""
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".license") {
			licenseFile := filepath.Join(execFolder, file.Name())
			ownerName, err = queryOwnerOfLicense(insightServerAddress, licenseFile)
			if err != nil {
				continue
			}
			break
		}
	}

	if ownerName == "" {
		err = fmt.Errorf("No valid license file found!")
		return "", err
	}

	return ownerName, nil
}

func queryOwnerOfLicense(insightServerAddress, licenseFile string) (string, error) {
	request, err := newfileUploadRequest(insightServerAddress + "/license-check", "file", licenseFile)
	if err != nil {
		return "", fmt.Errorf("Error while creating new file upload request: %v", err)
	}
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("Client do request failed! Request: %v. Error message: %v", request, err)
	}

	body := &bytes.Buffer{}
	_, err = body.ReadFrom(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Fatal error while reading from response body: %v", err)
	}
	resp.Body.Close()

	// Decode the JSON in the response
	var licenseCheck insight_server.LicenseCheckResponse
	if err := json.NewDecoder(body).Decode(&licenseCheck); err != nil {
		return "", fmt.Errorf("Error while deserializing license check response body! Error message: %v", err)
	}

	if !licenseCheck.Valid {
		return "", fmt.Errorf("License: %v is invalid! Although owner name is %v", licenseFile, licenseCheck.OwnerName)
	}

	return licenseCheck.OwnerName, nil
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

	req, err := http.NewRequest("POST", uri, body)
	if err != nil {
		return nil, fmt.Errorf("Failed to create new request! Error message: %v", err)
	}

	req.Header.Add("Content-Type", writer.FormDataContentType())

	return req, nil
}
