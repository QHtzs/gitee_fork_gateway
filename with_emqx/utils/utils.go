package utils

import (
	"encoding/xml"
	"io"
	"io/ioutil"
	"log"
	"os"
)

var (
	CfgInstance *XMlCnf = initConfig("setting.xml")

	Logger *log.Logger = genLogger("log.log")
)

func genLogger(file_name string) *log.Logger {
	fd, err := os.OpenFile(file_name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	muti_write := io.MultiWriter(fd, os.Stdout)
	l := log.New(muti_write, "", log.Ldate|log.Ltime|log.Lshortfile)
	return l
}

type MongoCnf struct {
	Name       xml.Name `xml:"mongo"`
	Host       string   `xml:"host"`
	Port       string   `xml:"port"`
	User       string   `xml:"user"`
	Pwd        string   `xml:"pwd"`
	Database   string   `xml:"database"`
	AuthSource string   `xml:"authsource"`
}

type MqttCnf struct {
	Name     xml.Name `xml:"mqtt"`
	Host     string   `xml:"host"`
	Port     string   `xml:"port"`
	User     string   `xml:"user"`
	Pwd      string   `xml:"pwd"`
	ClientId string   `xml:"clientid"`
}

type WssCnf struct {
	Name    xml.Name `xml:"wss"`
	Private string   `xml:"crt"`
	Public  string   `xml:"key"`
	Domain  string   `xml:"domain"`
	Path    string   `xml:"path"`
	Port    string   `xml:"port"`
	Type    string   `xml:"type"`
}

type TcpCnf struct {
	Name   xml.Name `xml:"tcp"`
	Serial string   `xml:"name"`
	Type   string   `xml:"type"`
	Port   string   `xml:"port"`
}

type XMlCnf struct {
	Name      xml.Name `xml:"config"`
	Mongo     MongoCnf `xml:"mongo"`
	Mqtt      MqttCnf  `xml:"mqtt"`
	Wss       WssCnf   `xml:"wss"`
	Tcp       []TcpCnf `xml:"tcp"`
	StatusUrl string   `xml:"StatusUrl"`
}

func initConfig(filename string) *XMlCnf {
	cfg := &XMlCnf{}
	file, err := os.Open(filename)
	if err != nil {
		panic("配置文件读取失败，请确保配置文件存在且路径正确:" + err.Error())
	}
	defer file.Close()
	xml_bytes, err := ioutil.ReadAll(file)
	if err != nil {
		panic("配置文件读取失败，请确保配置文件存在且路径正确:" + err.Error())
	}
	err = xml.Unmarshal(xml_bytes, cfg)
	return cfg
}
