package main

/*
读取xml配置文件
auth: TTG
*/

import (
	"encoding/xml"
	"io/ioutil"
	"os"
)

type TtgTcpPorts struct {
	XMLName   xml.Name `xml:"Ports"`
	WebClient string   `xml:"WebClient"`
	GateWay   string   `xml:"GateWay"`
	Control   string   `xml:"Control"`
	UdpPort   string   `xml:"UdpPort"`
	WsPort    string   `xml:"WsPort"`
}

type TtgBeatPackages struct {
	XMLName   xml.Name `xml:"BeatPackages"`
	WebClient string   `xml:"WebClient"`
	GateWay   string   `xml:"GateWay"`
	Control   string   `xml:"Control"`
}

type TtgRedisCfg struct {
	XMLName    xml.Name `xml:"Redis"`
	RedisUrl   string   `xml:"RedisURL"`
	PassWord   string   `xml:"PassWord"`
	MaxIdle    int      `xml:"MaxIdle"`
	TimeoutSec int64    `xml:"TimeoutSec"`
	MaxActive  int      `xml:"MaxActive"`
	Wait       bool     `xml:"Wait"`
}

type TtgOtherCfg struct {
	XMLName     xml.Name `xml:"Other"`
	TimeOut     int64    `xml:"TcpTimeout"`
	ConfirmSize int      `xml:"TcpConfSize"`
	StatusUrl   string   `xml:"StatusUrl"`
	ProUrl      string   `xml:"ProUrl"`
}

type TtgEncrypt struct {
	XMLName xml.Name `xml:"Encrypt"`
	GateWay bool     `xml:"GateWay"`
}

type HttpsConf struct {
	XMLName xml.Name `xml:"Https"`
	Crt     string   `xml:"Crt"`
	Key     string   `xml:"Key"`
}

type Configure struct {
	XMLName      xml.Name        `xml:"servers"`
	Https        HttpsConf       `xml:"Https"`
	Ports        TtgTcpPorts     `xml:"Ports"`
	BeatPackages TtgBeatPackages `xml:"BeatPackages"`
	RedisCfg     TtgRedisCfg     `xml:"Redis"`
	Other        TtgOtherCfg     `xml:"Other"`
	NeedEncrypt  TtgEncrypt      `xml:"Encrypt"`
	LogFilePath  string          `xml:"LogFilePath"`
	PProfDebuger bool            `xml:"PProfDebuger"`
}

func load_config(filename string) Configure {
	cfg := Configure{}
	file, err := os.Open(filename)
	if err != nil {
		panic("配置文件读取失败，请确保配置文件存在且路径正确:" + err.Error())
	}
	defer file.Close()
	xml_bytes, err := ioutil.ReadAll(file)
	if err != nil {
		panic("配置文件读取失败，请确保配置文件存在且路径正确:" + err.Error())
	}
	err = xml.Unmarshal(xml_bytes, &cfg)
	return cfg
}

var ConfigInstance Configure = load_config("conf.xml")
