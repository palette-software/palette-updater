package main

import (
	"errors"
	"fmt"
	wmi "github.com/StackExchange/wmi"
	log "github.com/palette-software/insight-tester/common/logging"
	svcControl "github.com/palette-software/insight-tester/common/service_control"
	servdis "github.com/palette-software/palette-updater/services-discovery"
	"os"
	"os/exec"
	"path/filepath"
)

var Agent = "PaletteInsightAgent"
var BatchFile = "reinstall.bat"

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
	return serviceControl.Stop(Agent)
	// var dst []Win32_Service
	// q := wmi.CreateQuery(&dst, "where Name like '%Palette%'")
	// log.Info.Println("Query: ", q)
	// err := wmi.Query(q, &dst)
	// for _, srv := range dst {
	// pathName := stripPathName(srv.PathName)
	// log.Info.Printf("wtf: %s\n", pathName)
	// log.Info.Printf("Version: %s\n", getVersion(pathName))
	// }
	// return err
}

func createBatchFile(msiPath string, targetDir string) error {
	f, err := os.Create(BatchFile)
	if err != nil {
		return err
	}
	defer f.Close()
	reinstallCommand := fmt.Sprintf("msiexec /i \"%s\" INSTALLFOLDER=\"%s\" /qn", msiPath, targetDir)
	_, err = f.WriteString(reinstallCommand)
	if err != nil {
		return err
	}
	return nil
}

func deleteBatchFile() {
	os.Remove(BatchFile)
}

func reinstallServices(msiPath string) error {
	var dst []servdis.Win32_Service
	q := wmi.CreateQuery(&dst, "where Name like '%Palette%'")
	err := wmi.Query(q, &dst)
	if err != nil {
		return err
	}
	var targetDir string = ""
	for _, srv := range dst {
		targetDir = filepath.Dir(servdis.StripPathName(srv.PathName))
	}
	if targetDir == "" {
		return errors.New("Could not find installed agent.")
	}
	err = createBatchFile(msiPath, targetDir)
	if err != nil {
		return err
	}
	cmd := exec.Command(BatchFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	deleteBatchFile()
	return err
}

func startServices(serviceControl svcControl.ServiceControl) error {
	return serviceControl.Start(Agent)
}

func main() {
	// Initialize the log to write into file instead of stderr
	// open output file
	os.Mkdir("Logs", 777)
	logFileName := "Logs/updater.log"
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

	log.Info.Printf("Firing up updater... Command line %s", os.Args)

	if len(os.Args) < 2 {
		log.Error.Printf("Usage: %s installer_file\n", os.Args[0])
		os.Exit(1)
	}
	installerFile := os.Args[1]

	log.Info.Println("Checking prerequisites.")
	err = checkUpdate(os.Args[1])
	if err != nil {
		log.Error.Printf("Stopping update as could not validate update package: %s", err)
		os.Exit(1)
	}

	log.Info.Println("Stopping services.")
	var serviceControl svcControl.ServiceControl
	err = stopServices(serviceControl)
	if err != nil {
		// Should not stop here. Service needs to be started anyway from now on.
		log.Warning.Printf("Could not stop service: %s", err)
	}

	log.Info.Println("Reinstalling services.")
	err = reinstallServices(installerFile)
	if err != nil {
		// Should not stop here. Service needs to be started anyway from now on.
		log.Warning.Printf("Failed to install service: %s", err)
	}

	log.Info.Println("Restarting services.")
	err = startServices(serviceControl)
	// When we get error here we should try again....
}
