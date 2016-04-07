// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package service_control

import (
	"fmt"
	"os"
	"path/filepath"

	//"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
	"golang.org/x/sys/windows/svc"
)

func exePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			err = fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
}

func installService(name, displayName, description string) error {
	exepath, err := exePath()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	service, err := m.OpenService(name)
	if err == nil {
		service.Close()
		return fmt.Errorf("service %s already exists", name)
	}
	serviceConfig := mgr.Config{
		DisplayName:  displayName,
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
		Description:  description,
	}
	service, err = m.CreateService(name, exepath, serviceConfig, "is", "auto-started")
	if err != nil {
		return err
	}
	defer service.Close()
	//err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	//if err != nil {
	//	s.Delete()
	//	return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	//}
	return nil
}

func removeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return err
	}
	//err = eventlog.Remove(name)
	//if err != nil {
	//	return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
	//}
	return nil
}

func queryService(name string) (svc.Status, error) {
	m, err := mgr.Connect()
	if err != nil {
		return svc.Status{}, err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return svc.Status{}, fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()
	return s.Query()
}
