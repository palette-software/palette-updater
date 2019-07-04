package common

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kardianos/osext"
	log "github.com/palette-software/go-log-targets"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

func setupLogRotate(logFilePath string) (*lumberjack.Logger, error) {
	logRotate := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10, // megabytes
		MaxBackups: 10,
	}

	return logRotate, log.AddTarget(logRotate, log.LevelDebug)
}

func setupSplunkLogger(execFolder, logFilename string) (*log.SplunkTarget, error) {
	license, err := GetLicenseData(execFolder)
	if err == nil {
		log.Info("Owner of the license:", license.Owner)
		// Add logging to Splunk as well
		splunkLogger, err := log.NewSplunkTarget(SplunkServerAddress, WatchdogSplunkToken, license.Owner)
		if err == nil {
			log.AddTarget(splunkLogger, log.LevelDebug)
			return splunkLogger, nil
		} else {
			log.Error("Failed to create Splunk target for ", logFilename, "! Error: ", err)
			return nil, err
		}
	} else {
		log.Error("Failed to get license data in ", logFilename, "! Continue without Splunk logging! Error: ", err)
		return nil, err
	}
}

// Initialize the log to write into file instead of stderr
func InitLogging(logFilename string) (func(), error) {
	// Do not use relative paths, otherwise our files will end up in Windows/System32
	execFolder, errorToLogLater := osext.ExecutableFolder()
	if errorToLogLater != nil {
		execFolder = ""
	}

	// open output file
	logsFolder := filepath.Join(execFolder, "Logs")
	err := os.Mkdir(logsFolder, 0777)
	if err != nil {
		fmt.Println("Failed to create log folder! ", err)
		panic(err)
	}

	logFilePath := filepath.Join(logsFolder, logFilename)

	logRotate, err := setupLogRotate(logFilePath)
	if err != nil {
		return nil, err
	}

	splunkLogger, err := setupSplunkLogger(execFolder, logFilename)
	if err != nil {
		return nil, err
	}

	if errorToLogLater != nil {
		log.Error("Failed to retrieve executable folder, thus base dir is not set! Error message: ",
			errorToLogLater)
	}

	closeLogging := func() {
		if err := logRotate.Close(); err != nil {
			fmt.Println("Failed to close log file! ", err)
		}
		if err := splunkLogger.Close(); err != nil {
			fmt.Println("Failed to close splunk logger! ", err)
		}
	}

	return closeLogging, nil
}
