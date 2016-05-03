package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	insight "github.com/palette-software/insight-server/lib"
	log "github.com/palette-software/insight-tester/common/logging"

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
	// FIXME: tenant=default needs a real tenant in the future
	resp, err := http.Get(insightServerAddress + "/commands/recent?tenant=default")
	if err != nil {
		log.Errorf("Error during querying recent command: ", err)
		return err
	}
	log.Debugf("Recent command response: %s", resp)
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err = fmt.Errorf("Getting recent command failed! Server response: %s", resp)
		log.Error(err)
		return err
	}

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

	err = performCommand(command.Cmd)
	if err != nil {
		log.Error("Failed to perform command! Error message: ", err)
		return err
	}

	pws.lastPerformedCommand = command
	return err
}
