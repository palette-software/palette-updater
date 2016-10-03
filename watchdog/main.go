//Copyright (c) 2009 The Go Authors. All rights reserved.
// (only for this file of the project)
//
//Redistribution and use in source and binary forms, with or without
//modification, are permitted provided that the following conditions are
//met:
//
//* Redistributions of source code must retain the above copyright
//notice, this list of conditions and the following disclaimer.
//* Redistributions in binary form must reproduce the above
//copyright notice, this list of conditions and the following disclaimer
//in the documentation and/or other materials provided with the
//distribution.
//* Neither the name of Google Inc. nor the names of its
//contributors may be used to endorse or promote products derived from
//this software without specific prior written permission.
//
//THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
//"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
//LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
//A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
//OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
//SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
//LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
//DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
//THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
//(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
//OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
//

package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/palette-software/insight-tester/common/logging"
	"github.com/palette-software/palette-updater/common"
	svcControl "github.com/palette-software/palette-updater/service_control"

	"github.com/kardianos/osext"
	"golang.org/x/sys/windows/svc"
)

const shutdownTimer = 10 * time.Second

// This mutex prevents starting the agent service during agent update, because the service
// needs to be in a stopped state while uninstalling the agent service, otherwise a system
// reboot might be required, which is really not desired.
var agentSvcMutex sync.Mutex

// Prints usage information
func usage(errormsg string) {
	fmt.Fprintf(os.Stderr,
		"%s\n\n"+
			"usage: %s <command>\n"+
			"       where <command> is one of\n"+
			"       install, remove, debug, start or stop.\n",
		errormsg, os.Args[0])
	os.Exit(2)
}

// Global variables for proper working directories and paths
var logsFolder, updatesFolder, baseFolder string

func main() {
	// Do not use relative paths, otherwise our files will end up in Windows/System32
	execFolder, errorToLogLater := osext.ExecutableFolder()
	if errorToLogLater != nil {
		baseFolder = ""
	}

	// Set up our paths
	baseFolder = execFolder
	logsFolder = baseFolder + "/Logs"
	updatesFolder = baseFolder + "/Updates"

	// Initialize the log to write into file instead of stderr
	// open output file
	os.Mkdir(logsFolder, 777)
	logFileName := logsFolder + "/watchdog.log"
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Println("Failed to open log file! ", err)
		panic(err)
	}

	// close file on exit and check for its returned error
	defer func() {
		if err := logFile.Close(); err != nil {
			fmt.Println("Failed to close log file! ", err)
			panic(err)
		}
	}()
	log.AddTarget(logFile, log.LevelDebug)

	license, err := common.GetLicenseData(execFolder)
	if err == nil {
		log.Info("Owner of the license: ", license.Owner)
		// Add logging to Splunk as well
		splunkLogger, err := log.NewSplunkTarget(common.SplunkServerAddress, common.WatchdogSplunkToken, license.Owner)
		if err == nil {
			defer splunkLogger.Close()
			// Only Splunk target may block the shutdown, so this is the only case we need
			// to make sure that Watchdog shuts down in time.
			// IMPORTANT NOTE: the order of the deferred calls are important, because Splunk
			// target's Close() is blocking call!!
			defer shutdownInTime()
			log.AddTarget(splunkLogger, log.LevelDebug)
		} else {
			log.Error("Failed to create Splunk target for watchdog! Error: ", err)
		}
	} else {
		log.Error("Failed to get license owner for watchdog! Error:", err)
	}

	log.Infof("Firing up %s... Command line %s", common.WatchdogSvcDisplayName, os.Args)

	if errorToLogLater != nil {
		log.Error("Failed to retrieve executable folder, thus base dir is not set! Error message: ",
			errorToLogLater)
	}

	log.Info("Base folder is: ", baseFolder)

	if len(os.Args) < 2 {
		usage("no command specified")
	}

	// Instantiate the service controller
	var serviceControl svcControl.ServiceControl

	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "debug":
		runService(common.WatchdogSvcName, true)
	case "install":
		err = serviceControl.Install(common.WatchdogSvcName, common.WatchdogSvcDisplayName, common.WatchdogSvcDescription)
	case "remove":
		err = serviceControl.Remove(common.WatchdogSvcName)
	case "start":
		err = serviceControl.Start(common.WatchdogSvcName)
	case "stop":
		err = serviceControl.Stop(common.WatchdogSvcName)
	case "is":
		// In this case there needs to be more command line arguments, such as "auto-started"
		if len(os.Args) < 3 {
			log.Errorf("Unexpected end of command line arguments! Command line arguments: %s", os.Args)
		} else {
			cmdSecond := strings.ToLower(os.Args[2])
			switch cmdSecond {
			case "auto-started", "manual-started":
				isIntSess, err := svc.IsAnInteractiveSession()
				if err != nil {
					log.Fatalf("failed to determine if we are running in an interactive session: %v", err)
				}
				if !isIntSess {
					runService(common.WatchdogSvcName, false)
				} else {
					err = serviceControl.Start(common.WatchdogSvcName)
				}
			default:
				log.Error("Unexpected comamnd after \"is\": ", cmdSecond)
			}
		}
	default:
		log.Fatalf("Invalid startup command argument: \"%s\"", cmd)
		usage(fmt.Sprintf("invalid command %s", cmd))
	}

	if err != nil {
		log.Fatalf("failed to %s %s: %v", cmd, common.WatchdogSvcName, err)
		return
	}

	log.Infof("Command %s execution finished.", os.Args)
	return
}

// Make sure that Watchdog stops in a timely fashion. This is really important because it is
// running as a service, and if the service fails to stop it can mess up our Palette Insight
// Agent update process.
func shutdownInTime() {
	go func() {
		tickShutdown := time.Tick(shutdownTimer)
		select {
		case <-tickShutdown:
			// It is necessary to shut down normally in a timely fashion
			log.Error("Watchdog shutdown timed out! Force quitting.")
			os.Exit(0)
		}
	}()
}
