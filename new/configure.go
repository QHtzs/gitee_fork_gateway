package main

/*
@brief:读取xml配置文件并解析
@author: TTG
*/

import (
	"encoding/xml"
	"io/ioutil"
	"os"
)

//端口配置项
type TtgTcpPorts struct {
	XMLName   xml.Name `xml:"Ports"`
	WebClient string   `xml:"WebClient"`
	GateWay   string   `xml:"GateWay"`
	Control   string   `xml:"Control"`
	UdpPort   string   `xml:"UdpPort"`
	WsPort    string   `xml:"WsPort"`
}

//心跳配置项
type TtgBeatPackages struct {
	XMLName   xml.Name `xml:"BeatPackages"`
	WebClient string   `xml:"WebClient"`
	GateWay   string   `xml:"GateWay"`
	Control   string   `xml:"Control"`
}

//redis配置项
type TtgRedisCfg struct {
	XMLName    xml.Name `xml:"Redis"`
	RedisUrl   string   `xml:"RedisURL"`
	PassWord   string   `xml:"PassWord"`
	MaxIdle    int      `xml:"MaxIdle"`
	TimeoutSec int64    `xml:"TimeoutSec"`
	MaxActive  int      `xml:"MaxActive"`
	Wait       bool     `xml:"Wait"`
}

//其它配置项
type TtgOtherCfg struct {
	XMLName     xml.Name `xml:"Other"`
	TimeOut     int64    `xml:"TcpTimeout"`
	ConfirmSize int      `xml:"TcpConfSize"`
	StatusUrl   string   `xml:"StatusUrl"`
	ProUrl      string   `xml:"ProUrl"`
}

//加密配置项
type TtgEncrypt struct {
	XMLName       xml.Name `xml:"Encrypt"`
	GateWay       bool     `xml:"GateWay"`
	GateWayPubkey string   `xml:"GateWayPubkey"`
}

// https服务(websocket)配置项
type HttpsConf struct {
	XMLName xml.Name `xml:"Https"`
	Crt     string   `xml:"Crt"`
	Key     string   `xml:"Key"`
}

// 配置
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
	Host         string          `xml:"Host"`
}

//配置文件解析函数
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
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	return cfg
}

//配置实例，全局变量
var ConfigInstance Configure = load_config("conf.xml")
