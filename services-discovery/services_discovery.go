package services_discovery

import (
	"fmt"
	wmi "github.com/StackExchange/wmi"
	log "github.com/palette-software/insight-tester/common/logging"
	"path/filepath"
	"strings"
)

type Win32_Service struct {
	Name        string
	PathName    string
	DisplayName string
	StartName   string
}

type CIM_DataFile struct {
	Name    string
	Version string
}

func StripPathName(fullName string) string {
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

func GetServiceVersion(serviceName string) (string, error) {
	var dst []Win32_Service
	whereClause := "where Name like '%" + serviceName + "%'"
	q := wmi.CreateQuery(&dst, whereClause)
	err := wmi.Query(q, &dst)
	if err != nil {
		log.Error.Printf("Failed to get version of service: %s. Error message: %s", serviceName, err)
		return "", err
	}
	for _, srv := range dst {
		pathName := StripPathName(srv.PathName)
		// FIXME: Is it working only for the first hit?
		return getVersion(pathName), nil
	}

	err = fmt.Errorf("Failed to look up version for service: %s. The service was not found.", serviceName)
	log.Error.Println(err)
	return "", err
}

func getVersion(pathName string) string {
	var fileData []CIM_DataFile
	cond := fmt.Sprintf("where Drive=\"%s\" and Path='%s' and Name like '%%%s%%'", getDrive(pathName), getPath(pathName), getExecutable(pathName))
	q := wmi.CreateQuery(&fileData, cond)
	log.Debug.Println("Query: ", q)
	_ = wmi.Query(q, &fileData)
	if len(fileData) == 0 {
		return ""
	}
	return fileData[0].Version
}

func getDrive(pathName string) string {
	return filepath.VolumeName(pathName)
}

func getPath(fullPath string) string {
	dir := filepath.Dir(fullPath)
	if strings.Contains(dir, ":") {
		dir = strings.Split(dir, ":")[1]
	}
	dir = strings.Replace(dir, "\\", "\\\\", -1)
	return dir + "\\\\"
}

func getExecutable(fullPath string) string {
	return filepath.Base(fullPath)
}
