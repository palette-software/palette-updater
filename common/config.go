package common

import (
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/palette-software/go-log-targets"

	"github.com/palette-software/insight-server/lib"
	"gopkg.in/yaml.v2"
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
