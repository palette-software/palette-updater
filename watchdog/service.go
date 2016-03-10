// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package main

import (
	"time"

	"encoding/json"
	"fmt"
	log "github.com/palette-software/insight-tester/common/logging"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"io/ioutil"
	"net/http"
)

// FIXME: Maybe the following Version struct might be reused from insight-server
// The base structure for a SemVer like version
type Version struct {
	// The version according to SemVer
	Major, Minor, Patch int
}

// Combines a version with an actual product and a file
type UpdateVersion struct {
	Version
	// The name of the product
	Product string
	// The Md5 checksum of this update
	Md5 string
	// The url where this update can be downloaded from
	Url string
}

// Compare versions
func (thisVersion *Version) IsNewer(otherVersion Version) bool {
	if thisVersion > otherVersion {
		return true
	}

	return false
}

func getLatestVersion(product string) (Version, error) {
	log.Debug.Printf("Getting latest %s version...", product)
	// FIXME: Get webservice address and port dynamically
	resp, err := http.Get("http://localhost:9000/updates/latest-version?product=agent")
	if err != nil {
		log.Error.Println("Error during querying latest agent version: ", err)
		return nil, err
	}
	log.Info.Printf("Latest %s version: %s", product, resp)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error.Println("Failed to read body of response: ", err)
		return nil, err
	}
	log.Debug.Printf("Body of response: %s", body)

	version := &Version{}
	if err := json.NewDecoder(body).Decode(version); err != nil {
		return nil, fmt.Errorf("Error while deserializing version response body '%s'. Error message: %v", body, err)
	}

	return version, nil
}

func getCurrentVersion(product string) (Version, error) {
	// FIXME: Find a way to determine the currently installed version of the given product
	return Version{0, 0, 0}, nil
}

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
			if getLatestVersion("agent") {

			}

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
