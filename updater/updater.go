package main

import (
    "os"
    "errors"
    log "github.com/palette-software/insight-tester/common/logging"
)

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

func stopServices() (error) {
    return errors.New("Not implemented")
}

func reinstallServices() (error) {
    return errors.New("Not implemented")
}

func startServices() (error) {
    return errors.New("Not implemented")
}

func main() {
    log.Init()
    if len(os.Args) < 2 {
        log.Error.Printf("Usage: %s update_location\n", os.Args[0])
        os.Exit(1)
    }

    log.Info.Println("Checking prerequisites.")
    err := checkUpdate(os.Args[1])
    if err != nil {
        log.Error.Printf("Stopping update as could not validate update package: %s", err)
        os.Exit(1)
    }
    
    log.Info.Println("Stopping services.")
    err = stopServices()
    if err != nil {
        log.Error.Printf("Stopping update as could not validate update package: %s", err)
        os.Exit(1)
    }

    log.Info.Println("Reinstalling services.")
    err = reinstallServices()
    // Should not stop here. Service needs to be started anyway from now on.

    log.Info.Println("Restarting services.")
    err = startServices()
    // When we get error here we should try again....
}
