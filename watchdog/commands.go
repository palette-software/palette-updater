package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	insight "github.com/palette-software/insight-server/lib"
	log "github.com/palette-software/insight-tester/common/logging"
	"github.com/palette-software/palette-updater/common"

	gocp "github.com/cleversoap/go-cp"
)

// FIXME: .String() function should be added to insight-server, until then we use this function.
func commandToString(cmd insight.AgentCommand) string {
	return fmt.Sprintf("{\"timestamp\":\"%s\", \"command\":\"%s\"}", cmd.Ts, cmd.Cmd)
}

func performCommand(arguments ...string) (err error) {
	tempUpdaterFileName := filepath.Join(baseFolder, "manager_in_action.exe")
	err = gocp.Copy(filepath.Join(baseFolder, "manager.exe"), tempUpdaterFileName)
	if err != nil {
		log.Error("Failed to make copy of manager.exe! Error message: ", err)
		return err
	}
	defer func() {
		log.Debug("Deleting ", tempUpdaterFileName)
		err = os.Remove(tempUpdaterFileName)
		if err != nil {
			log.Errorf("Failed to delete %s! Error message: %s", tempUpdaterFileName, err)
		}
	}()

	log.Infof("Performing command: %s", arguments)
	cmd := exec.Command(tempUpdaterFileName, arguments...)
	agentSvcMutex.Lock()
	defer agentSvcMutex.Unlock()
	err = cmd.Run()
	if err != nil {
		log.Errorf("Failed to execute %s! Error message: %s", tempUpdaterFileName, err)
		return err
	}

	log.Infof("Successfully performed command: %s", arguments)
	return nil
}

func (pws *paletteWatchdogService) checkForCommand(insightServerAddress string) error {
	client, err := common.NewApiClient(baseFolder)
	if err != nil {
		log.Error("Failed to create Insight API client while checking for command! Error: ", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		log.Error("Failed to get hostname for command check! Error: ", err)
		return err
	}
	resp, err := client.Get(fmt.Sprint("/command?hostname=", url.QueryEscape(hostname)))
	if err != nil {
		// The error has already been logged
		return err
	}
	log.Debugf("Recent command response: %v", resp)
	defer resp.Body.Close()

	// Decode the JSON in the response
	var command insight.AgentCommand
	if err := json.NewDecoder(resp.Body).Decode(&command); err != nil {
		log.Errorf("Error while deserializing command response body. Error message: %v", err)
		return err
	}

	log.Info("Recent command: ", commandToString(command))
	if pws.lastPerformedCommand == command {
		// Command has already been performed. Nothing to do now.
		log.Debugf("Command %s has already been performed.", commandToString(command))
		return nil
	}

	cmdTimestamp, err := time.Parse(time.RFC3339, command.Ts)
	if err != nil {
		log.Errorf("Failed to parse command timestamp: %s! Error message: %s", command.Ts, err)
		return err
	}

	if cmdTimestamp.Add(7 * time.Minute).Before(time.Now()) {
		log.Debugf("Command %s is not recent enough. Ignore it.",
			commandToString(command))
		return nil
	}

	switch command.Cmd {
	case "start", "stop":
		err = performCommand(command.Cmd)
		if err != nil {
			log.Errorf("Failed to perform command: '%s'! Error message: %v", command.Cmd, err)
			return err
		}
	case "GET-CONFIG":
		err = performGetConfig(client, hostname)
		if err != nil {
			log.Error("Failed to get and apply new config file! Error: ", err)
			return err
		}

	case "PUT-CONFIG":
		log.Info("Uploading agent's config file...")
		agentConfigPath, err := common.FindAgentConfigFile(baseFolder)
		if err != nil {
			return err
		}

		err = client.UploadFile(fmt.Sprint("/config?hostname=", url.QueryEscape(hostname)), agentConfigPath)
		if err != nil {
			return err
		}
		log.Info("Successfully uploaded agent's config file: ", agentConfigPath)
	default:
		err = fmt.Errorf("Unknown command received: %v", command.Cmd)
		log.Error(err)
		return err
	}

	pws.lastPerformedCommand = command
	return err
}

func performGetConfig(client *common.ApiClient, hostname string) error {
	log.Info("Acquiring remote config...")
	// Create a temporary folder for incoming config file and delete it after reconfiguration is done
	incomingConfigFolder := path.Join(baseFolder, "incoming-config")
	defer os.RemoveAll(incomingConfigFolder)

	destinationPath := path.Join(incomingConfigFolder, "Config.yml")
	err := client.DownloadFile(fmt.Sprint("/config?hostname=", url.QueryEscape(hostname)), destinationPath)
	if err != nil {
		return err
	}

	// Make sure that the downloaded Config.yml is correct and contains the required fields
	newConfig, err := common.ParseConfig(destinationPath)
	if err != nil {
		return err
	}

	license, err := common.GetLicenseDataForConfig(newConfig)
	if err != nil {
		return err
	}

	if !license.Valid {
		err = fmt.Errorf("License is invalid in new conifg file: '%s'! License information: %v",
			destinationPath, license)
		log.Error(err)
		return err
	}

	// Overwrite Insight Agent's current config file
	currentConfigPath, err := common.FindAgentConfigFile(baseFolder)
	if err != nil {
		return err
	}
	os.Rename(destinationPath, currentConfigPath)

	log.Info("Successfully acquired and applied remote config file.")
	return nil
}
