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

	insight "github.com/palette-software/insight-server"
	log "github.com/palette-software/insight-tester/common/logging"
	"github.com/palette-software/palette-updater/common"
	svcControl "github.com/palette-software/palette-updater/service_control"

	"github.com/kardianos/osext"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
)

// Timer constants
const updateTimer = 3 * time.Minute
const commandTimer = 2 * time.Minute
const aliveTimer = 5 * time.Minute

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

// Defining the watchdog service
type paletteWatchdogService struct{
	lastPerformedCommand insight.AgentCommand
}

func (pws *paletteWatchdogService) Execute(args []string, changeRequest <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}
	tickUpdate := time.Tick(updateTimer)
	tickCommand := time.Tick(commandTimer)
	tickAlive := time.Tick(aliveTimer)
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
loop:
	for {
		select {
		case <-tickUpdate:
			// Do the checks in a different thread so that the main thread may remain responsive
			go func() {
				// Remove the updates folder to make sure the disk is not going to filled
				// with orphaned update files
				os.RemoveAll(updatesFolder)

				//checkForUpdates("updater")
				//checkForUpdates("watchdog")
				checkForUpdates("agent")
			}()

		case <-tickCommand:
			// Do the checks in a different thread so that the main thread may remain responsive
			go func() {
				pws.checkForCommand()
			}()

		case <-tickAlive:
			go func () {
				if pws.lastPerformedCommand.Cmd == "stop" {
					log.Debug.Printf("Skipped alive check for %s, since it is commanded to be stopped.", common.AgentSvcName)
					return
				}
				var serviceControl svcControl.ServiceControl
				svcStatus, err := serviceControl.Query(common.AgentSvcName)
				if err != nil {
					log.Error.Printf("Failed to query status of service: %s! Error message: %v", common.AgentSvcName, err)
					return
				}

				// Restart the agent service if it is not running and it is not commanded to stop
				if svcStatus.State == svc.Stopped {
					agentSvcMutex.Lock()
					defer agentSvcMutex.Unlock()
					serviceControl.Start(common.AgentSvcName)
					log.Info.Printf("Watchdog found %s in stopped state. Restarted it.", common.AgentSvcName)
				} else {
					log.Info.Printf("%s is still alive. (Service state: %d)", common.AgentSvcName, svcStatus.State)
				}
			}()

		case cr := <-changeRequest:
			switch cr.Cmd {
			case svc.Interrogate:
				changes <- cr.CurrentStatus
				// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- cr.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Info.Printf("Stopping %s...", common.WatchdogSvcDisplayName)
				break loop
			default:
				log.Error.Printf("unexpected control request #%d", cr)
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func runService(name string, isDebug bool) {
	var err error

	log.Info.Printf("starting %s service", name)
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &paletteWatchdogService{})
	if err != nil {
		log.Error.Printf("%s service failed: %v", name, err)
		return
	}
	log.Info.Printf("%s service stopped", name)
}

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

	// Set the levels to be ignored to ioutil.Discard
	// Levels:  DEBUG,   INFO,    WARNING, ERROR,   FATAL
	log.InitLog(logFile, logFile, logFile, logFile, logFile)

	log.Info.Printf("Firing up %s... Command line %s", common.WatchdogSvcDisplayName, os.Args)

	if errorToLogLater != nil {
		log.Error.Println("Failed to retrieve executable folder, thus base dir is not set! Error message: ",
			errorToLogLater)
	}

	log.Info.Println("Base folder is: ", baseFolder)

	if len(os.Args) < 2 {
		usage("no command specified")
	}

	// Instantiate the service controller
	var serviceControl svcControl.ServiceControl

	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "debug":
		// FIXME: runService is not platform-independent
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
			log.Error.Printf("Unexpected end of command line arguments! Command line arguments: %s", os.Args)
		} else {
			cmdSecond := strings.ToLower(os.Args[2])
			switch cmdSecond {
			case "auto-started", "manual-started":
				isIntSess, err := svc.IsAnInteractiveSession()
				if err != nil {
					log.Fatalf("failed to determine if we are running in an interactive session: %v", err)
				}
				if !isIntSess {
					// FIXME: runService is not platform-independent
					runService(common.WatchdogSvcName, false)
				} else {
					err = serviceControl.Start(common.WatchdogSvcName)
				}
			default:
				log.Error.Println("Unexpected comamnd after \"is\": ", cmdSecond)
			}
		}

	// FIXME: Delete this section as it is only for debugging purposes.
	case "get":
		// Remove the updates folder to make sure the disk is not going to filled
		// with orphaned update files
		err = os.RemoveAll(updatesFolder)

		//checkForUpdates("updater")
		//checkForUpdates("watchdog")
		checkForUpdates("agent")
	// FIXME: End of debugging

	default:
		log.Fatalf("Invalid startup command argument: \"%s\"", cmd)
		usage(fmt.Sprintf("invalid command %s", cmd))
	}

	if err != nil {
		log.Fatalf("failed to %s %s: %v", cmd, common.WatchdogSvcName, err)
		return
	}

	log.Info.Printf("Command %s execution finished.", os.Args)
	return
}
