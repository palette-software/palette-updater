// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package service_control

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// IMPORTANT: it is critical that this timeout value is significantly larger, than
// the value used as shutdownTimer defined in watchdog/main.go. Not doing so, it
// may result in a failure during service uninstall, which may result in system reboot
// and the Palette Insight Agent will remain uninstalled at the end of the process!
const serviceControlTimeout = 30 * time.Second

func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	err = s.Start("is", "manual-started")
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

func controlService(name string, c svc.Cmd, to svc.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeout := time.Now().Add(serviceControlTimeout)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
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
