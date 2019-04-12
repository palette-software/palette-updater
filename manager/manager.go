package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/palette-software/go-log-targets"
	"github.com/palette-software/palette-updater/common"
	svcControl "github.com/palette-software/palette-updater/service_control"
	servdis "github.com/palette-software/palette-updater/services-discovery"

	"github.com/StackExchange/wmi"
)

const BatchFile = "reinstall.bat"

// Returns whether the given file or directory exists or not
func doesExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// Checks if the update is OK. This practically means checking if it exists for now.
func checkUpdate(updateLocation string) error {
	dirExists, err := doesExist(updateLocation)
	if err != nil || dirExists {
		return err
	}
	return nil
}

func stopServices(serviceControl svcControl.ServiceControl) error {
	return serviceControl.Stop(common.AgentSvcName)
	// var dst []Win32_Service
	//whereClause := "where Name like '%" + Agent + "%'"
	//log.Debug.Println("Discovering service where: ", whereClause)
	// q := wmi.CreateQuery(&dst, whereClause)
	// log.Info.Println("Query: ", q)
	// err := wmi.Query(q, &dst)
	// for _, srv := range dst {
	// pathName := stripPathName(srv.PathName)
	// log.Info.Printf("wtf: %s\n", pathName)
	// log.Info.Printf("Version: %s\n", getVersion(pathName))
	// }
	// return err
}

func createBatchFile(msiPath string, targetDir, installerLogFile string) error {
	f, err := os.Create(BatchFile)
	if err != nil {
		return err
	}
	defer f.Close()
	reinstallCommand := fmt.Sprintf("msiexec /i \"%s\" /norestart INSTALLFOLDER=\"%s\" /qnlv /log \"%s\"", msiPath, targetDir, installerLogFile)
	_, err = f.WriteString(reinstallCommand)
	if err != nil {
		return err
	}
	return nil
}

func reinstallServices(msiPath string) error {
	var dst []servdis.Win32_Service
	whereClause := "where Name like '%" + common.AgentSvcName + "%'"
	log.Debug("Discovering service where: ", whereClause)
	q := wmi.CreateQuery(&dst, whereClause)
	err := wmi.Query(q, &dst)
	if err != nil {
		return err
	}

	// Hopefully there will only be one target directory, but if get more for some reason, try them all.
	var targetDir string = ""
	for _, srv := range dst {
		targetDir = filepath.Dir(servdis.StripPathName(srv.PathName))
		log.Info("Found possible target dir: ", targetDir)

		if targetDir == "" {
			err = errors.New("Could not find installed agent.")
			continue
		}
		installerLogFile := fmt.Sprintf("%s\\Logs\\installer.log", targetDir)
		err = createBatchFile(msiPath, targetDir, installerLogFile)
		if err != nil {
			log.Warningf("Failed to create batch file with target dir: %s. Error message: %s", targetDir, err)
			continue
		}
		cmd := exec.Command(BatchFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		os.Remove(BatchFile)
		// Have the contents of the installer log in the common log
		installerResult, logErr := ioutil.ReadFile(installerLogFile)
		if logErr != nil {
			log.Errorf("Failed to read installer log file: %s", installerLogFile)
		} else {
			log.Infof("Contents of the installer.log file:\n%s", installerResult)
		}
		if err != nil {
			log.Warningf("Failed to execute batch file with target dir: %s. Error message: %s", targetDir, err)
			continue
		}

		// Successfully reinstalled services
		break
	}

	return err
}

func main() {
	closeLogging, err := common.InitLogging("manager.log")
	if err != nil {
		fmt.Println("Failed to init logging! ", err)
		panic(err)
	}

	defer closeLogging()

	log.Infof("Firing up manager... Command line %s", os.Args)

	if len(os.Args) < 2 {
		log.Errorf("Usage: %s installer_file\n", os.Args[0])
		os.Exit(1)
	}

	command := strings.ToLower(os.Args[1])
	var serviceControl svcControl.ServiceControl

	switch command {
	case "update":
		// In this case the following command-line argument is going to be
		// path for the update file.
		if len(os.Args) < 3 {
			log.Error("Missing update file for update command!")
			return
		}
		installerFile := os.Args[2]
		err = doUpdate(installerFile, serviceControl)
	case "start":
		err = serviceControl.Start(common.AgentSvcName)
	case "stop":
		err = serviceControl.Stop(common.AgentSvcName)
	default:
		log.Errorf("Unexpected command to execute: %s!", command)
		return
	}

	if err != nil {
		log.Errorf("Failed to execute command %s! Error message: %s", command, err)
		return
	}
}

func doUpdate(installerFile string, serviceControl svcControl.ServiceControl) error {
	log.Info("Checking prerequisites.")
	err := checkUpdate(installerFile)
	if err != nil {
		log.Errorf("Stopping update as could not validate update package: %s", err)
		os.Exit(1)
	}

	log.Info("Stopping services.")
	err = serviceControl.Stop(common.AgentSvcName)
	if err != nil {
		// Should not stop here. Service needs to be started anyway from now on.
		log.Warningf("Could not stop service: %s", err)
	}

	log.Info("Reinstalling services.")
	err = reinstallServices(installerFile)
	if err != nil {
		// Should not stop here. Service needs to be started anyway from now on.
		log.Warningf("Failed to install service: %s", err)
	}

	log.Info("Restarting services.")
	err = serviceControl.Start(common.AgentSvcName)
	// When we get error here we should try again....

	// Anyway, we need to make sure that the Watchdog is running after the reinstall.
	// These are going to be no-op commands if the watchdog is still running.
	// The following commands are going to be our safety belt.
	errWatchdog := serviceControl.Install(common.WatchdogSvcName, common.WatchdogSvcDisplayName, common.WatchdogSvcDescription)
	if errWatchdog != nil {
		if !strings.Contains(errWatchdog.Error(), "already exists") {
			log.Warningf("Failed to install %s. Error message: %s", common.WatchdogSvcDisplayName, err)
		}
	}

	errWatchdog = serviceControl.Start(common.WatchdogSvcName)
	if errWatchdog != nil {
		if !strings.Contains(errWatchdog.Error(), "already running") {
			log.Warningf("Failed to start %s. Error message: %s", common.WatchdogSvcDisplayName, err)
		}
	}

	return err
}
