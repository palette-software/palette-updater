package main

import (
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    log "github.com/palette-software/insight-tester/common/logging"
    svcControl "github.com/palette-software/insight-tester/common/service_control"
    wmi "github.com/StackExchange/wmi"
)

var Agent = "PaletteInsightAgent"

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

func checkUpdate(updateLocation string) (error) {
    dirExists, err := doesExist(updateLocation)
    if err != nil || dirExists {
        return err
    }
    return nil
}

type Win32_Service struct {
    Name string
    PathName string
    DisplayName string
    StartName string
}

type CIM_DataFile struct {
    Name string
    Version string
}

func stripPathName(fullName string) (string) {
    quote := "\""

    // Check if there is a quote in the first "word"
    firstWord := strings.Fields(fullName)[0]
    if !strings.Contains(firstWord, quote) {
        return firstWord
    }

    // Otherwise we should take the "word" after the first quote
    quoteSplitted := strings.Split(fullName, quote)
    return quoteSplitted[1]
}

func getDrive(pathName string) (string) {
    return filepath.VolumeName(pathName)
}

func getPath(fullPath string) (string) {
    dir := filepath.Dir(fullPath)
    if strings.Contains(dir, ":") {
        dir = strings.Split(dir, ":")[1]
    }
    dir = strings.Replace(dir, "\\", "\\\\", -1)
    return dir + "\\\\"
}

func getExecutable(fullPath string) (string) {
    return filepath.Base(fullPath)
}

func getVersion(pathName string) (string) {
    var fileData []CIM_DataFile
    cond := fmt.Sprintf("where Drive=\"%s\" and Path='%s' and Name like '%%%s%%'", getDrive(pathName), getPath(pathName), getExecutable(pathName))
    q := wmi.CreateQuery(&fileData, cond)
    log.Info.Println("Query: ", q)
    _ = wmi.Query(q, &fileData)
    if len(fileData) == 0 {
        return ""
    }
    return fileData[0].Version
}

func stopServices(serviceControl svcControl.ServiceControl) (error) {
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

func reinstallServices(msiPath string) (error) {
    var dst []Win32_Service
    q := wmi.CreateQuery(&dst, "where Name like '%Palette%'")
    log.Info.Println("Query: ", q)
    err := wmi.Query(q, &dst)
    if err != nil {
        return err
    }
    var targetDir string = ""
    for _, srv := range dst {
        targetDir = filepath.Dir(stripPathName(srv.PathName))
        targetDir = "C:\\1 2"
    }
    if targetDir == "" {
        return errors.New("Could not find installed agent.")
    }
    cmd := exec.Command("msiexec", "/i", msiPath, fmt.Sprintf(`INSTALLFOLDER="%s"`, targetDir), "/qn")
    // cmd := exec.Command("msiexec", "/i", msiPath, "/qn")
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    log.Info.Printf("About to run: %#v\n", cmd)
    log.Info.Printf("About to run: %s\n", cmd)
    return cmd.Run()
}

func startServices(serviceControl svcControl.ServiceControl) (error) {
    return serviceControl.Start(Agent)
}

func main() {
    log.Init()
    if len(os.Args) < 2 {
        log.Error.Printf("Usage: %s installer_file\n", os.Args[0])
        os.Exit(1)
    }
    installerFile := os.Args[1]

    log.Info.Println("Checking prerequisites.")
    err := checkUpdate(os.Args[1])
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
