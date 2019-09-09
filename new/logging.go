package main

import (
	"log"
	"os"
	"strings"
)

type privateLogHandle struct {
	file  string
	loger *log.Logger
}

func (p *privateLogHandle) WriteLog(v ...interface{}) {
	p.loger.Println(v...)
}

func singleton_log() privateLogHandle {
	hd := privateLogHandle{}
	name := strings.Trim(ConfigInstance.LogFilePath, " ")
	if name == "" {
		name = "server_log.log"
	}
	hd.file = name
	file, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("failed to open/create log file, exit!")
	}
	hd.loger = log.New(file, "", log.LstdFlags|log.Lshortfile)
	return hd
}

var LogHandle privateLogHandle = singleton_log()
