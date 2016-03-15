// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Example service program that beeps.
//
// The program demonstrates how to create Windows service and
// install / remove it on a computer. It also shows how to
// stop / start / pause / continue any service, and how to
// write to event log. It also shows how to use debug
// facilities available in debug package.
//

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	log "github.com/palette-software/insight-tester/common/logging"
	svcControl "github.com/palette-software/insight-tester/common/service_control"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
)

const svcDisplayName = "Palette Watchdog"

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
type paletteWatchdogService struct{}

func (pws *paletteWatchdogService) Execute(args []string, changeRequest <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}
	tick := time.Tick(5 * time.Second)
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
loop:
	for {
		select {
		case <-tick:
			checkForUpdates("agent")

		case cr := <-changeRequest:
			switch cr.Cmd {
			case svc.Interrogate:
				changes <- cr.CurrentStatus
				// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- cr.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Info.Printf("Stopping %s...", svcDisplayName)
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

func main() {
	const svcName = "palettewatchdog"

	// Initialize the log to write into file instead of stderr
	// open output file
	logFileName := os.Args[0] + ".log"
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Println("Failed to open log file! ", err)
		panic(err)
	}

	// close fo on exit and check for its returned error
	defer func() {
		if err := logFile.Close(); err != nil {
			fmt.Println("Failed to close log file! ", err)
			panic(err)
		}
	}()

	// Set the levels to be ignored to ioutil.Discard
	// Levels:  DEBUG,   INFO,    WARNING, ERROR,   FATAL
	log.InitLog(logFile, logFile, logFile, logFile, logFile)

	log.Debug.Printf("Starting up %s...", svcDisplayName)

	isIntSess, err := svc.IsAnInteractiveSession()
	if err != nil {
		log.Fatalf("failed to determine if we are running in an interactive session: %v", err)
	}
	if !isIntSess {
		// FIXME: runService is not platform-independent
		runService(svcName, false)
		return
	}

	if len(os.Args) < 2 {
		usage("no command specified")
	}

	// Instantiate the service controller
	var serviceControl svcControl.ServiceControl

	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "debug":
		// FIXME: runService is not platform-independent
		runService(svcName, true)
		return
	case "install":
		err = serviceControl.Install(svcName, "Palette Watchdog")
	case "remove":
		err = serviceControl.Remove(svcName)
	case "start":
		err = serviceControl.Start(svcName)
	case "stop":
		err = serviceControl.Stop(svcName)

	// FIXME: Delete this section as it is only for debugging purposes.
	case "get":
		checkForUpdates("agent")
	// FIXME: End of debugging

	default:
		usage(fmt.Sprintf("invalid command %s", cmd))
	}
	if err != nil {
		log.Fatalf("failed to %s %s: %v", cmd, svcName, err)
	}
	return
}
