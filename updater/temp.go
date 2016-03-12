package main

import (
    "fmt"
    "path/filepath"
    "strings"
    log "github.com/palette-software/insight-tester/common/logging"
    wmi "github.com/StackExchange/wmi"
)

func getServiceVersion(serviceName string) (string) {
    var dst []Win32_Service
    q := wmi.CreateQuery(&dst, "where Name like '%Palette%'")
    err := wmi.Query(q, &dst)
    if err != nil {
        return ""
    }
    for _, srv := range dst {
        pathName := stripPathName(srv.PathName)
        return getVersion(pathName)
    }
    return ""
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
