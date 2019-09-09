package utils

/*
auth:TTG
date 2019/7/15
*/

import (
	"log"
	"os"
	"strings"
)

type loggerHandle struct {
	File   string
	Logger *log.Logger
}

func (l *loggerHandle) WriteLog(v ...interface{}) {
	l.Logger.Println(v...)
}

func init_loghandle() loggerHandle {
	hd := loggerHandle{}
	name := strings.Trim(ConfigInstance.LogFilePath, " ")
	if name == "" {
		name = "log.log"
	}
	hd.File = name
	file, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		panic("failed to open/create log file, exit")
	}
	hd.Logger = log.New(file, "", log.LstdFlags|log.Lshortfile)
	return hd
}

var Logger loggerHandle = init_loghandle()
