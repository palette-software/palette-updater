package main

import (
	"os"
	"time"

	insight "github.com/palette-software/insight-server/lib"
	log "github.com/palette-software/insight-tester/common/logging"
	"github.com/palette-software/palette-updater/common"
	svcControl "github.com/palette-software/palette-updater/service_control"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
)

// Timer constants
const updateTimer = 3 * time.Minute
const commandTimer = 2 * time.Minute
const aliveTimer = 5 * time.Minute

// Defining the watchdog service
type paletteWatchdogService struct {
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

				checkForUpdates()
			}()

		case <-tickCommand:
			// Do the checks in a different thread so that the main thread may remain responsive
			go func() {
				pws.checkForCommand()
			}()

		case <-tickAlive:
			go func() {
				if pws.lastPerformedCommand.Cmd == "stop" {
					log.Debugf("Skipped alive check for %s, since it is commanded to be stopped.", common.AgentSvcName)
					return
				}
				var serviceControl svcControl.ServiceControl
				svcStatus, err := serviceControl.Query(common.AgentSvcName)
				if err != nil {
					log.Errorf("Failed to query status of service: %s! Error message: %v", common.AgentSvcName, err)
					return
				}

				// Restart the agent service if it is not running and it is not commanded to stop
				if svcStatus.State == svc.Stopped {
					agentSvcMutex.Lock()
					defer agentSvcMutex.Unlock()
					serviceControl.Start(common.AgentSvcName)
					log.Warningf("Watchdog found %s in stopped state. Restarted it.", common.AgentSvcName)
				} else {
					log.Infof("%s is still alive. (Service state: %d)", common.AgentSvcName, svcStatus.State)
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
				log.Infof("Stopping %s...", common.WatchdogSvcDisplayName)
				break loop
			default:
				log.Errorf("unexpected control request #%d", cr)
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func runService(name string, isDebug bool) {
	var err error

	log.Infof("starting %s service", name)
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &paletteWatchdogService{})
	if err != nil {
		log.Errorf("%s service failed: %v", name, err)
		return
	}
	log.Infof("%s service stopped", name)
}
