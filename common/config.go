package common

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	log "github.com/palette-software/insight-tester/common/logging"

	"gopkg.in/yaml.v2"
	"github.com/palette-software/insight-server/lib"
)

type Config struct {
	LicenseKey string     `yaml:"LicenseKey"`
	Webservice Webservice `yaml:"Webservice"`
}

type Webservice struct {
	Endpoint     string `yaml:"Endpoint"`
	UseProxy     bool   `yaml:"UseProxy"`
	ProxyAddress string `yaml:"ProxyAddress"`
}

func (w *Webservice) GetPreparedEndpoint() (string, error) {
	// Do the proxy setup, if necessary
	err := w.setupProxy()
	if err != nil {
		return "", err
	}

	return w.Endpoint, nil
}

func (w *Webservice) setupProxy() error {
	// Set the proxy address, if there is any
	if w.UseProxy {
		if len(w.ProxyAddress) == 0 {
			err := fmt.Errorf("Missing proxy address from config file!")
			log.Error(err)
			return err
		}
		proxyUrl, err := url.Parse(w.ProxyAddress)
		if err != nil {
			log.Errorf("Could not parse proxy settings: %s from %s. Error message: %s",
				w.ProxyAddress, insight_server.AgentConfigFileName, err)
			return err
		}
		http.DefaultTransport = &http.Transport{
			Proxy:           http.ProxyURL(proxyUrl),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		log.Info("Default Proxy URL is set to: ", proxyUrl)
	}

	return nil
}

func ParseConfig(configFilePath string) (Config, error) {
	var config Config

	configBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		log.Error("Error reading file: ", err)
		return config, err
	}

	// Parse the .yml config file
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		log.Error("Error parsing yaml: ", err)
		return config, err
	}

	return config, nil
}

func ParseAgentConfig(baseFolder string) (Config, error) {
	var config Config
	configFilePath, err := FindAgentConfigFile(baseFolder)
	if err != nil {
		return config, err
	}

	return ParseConfig(configFilePath)
}

func ObtainInsightServerAddress(baseFolder string) (string, error) {
	config, err := ParseAgentConfig(baseFolder)
	if err != nil {
		return "", err
	}

	insightServerAddress, err := config.Webservice.GetPreparedEndpoint()
	if err != nil {
		return "", err
	}

	return insightServerAddress, nil
}

// FIXME: locating the config file is not generic! This means this way is not going to be okay if we wanted to use this service as an auto-updater for the insight-server
// NOTE: This only works as long as the watchdog service runs from the very same folder as the agent.
// But they are supposed to be in the same folder by design.
func FindAgentConfigFile(baseFolder string) (string, error) {
	configPath := filepath.Join(baseFolder, "Config", insight_server.AgentConfigFileName)

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Error("Agent config file does not exist! Error message: ", err)
		return "", err
	}

	// Successfully located agent config file
	return configPath, nil
}
