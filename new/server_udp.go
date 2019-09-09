package main

import (
	"log"
	"net"
	"sync"
	"time"
)

type UdpServerEntity struct {
	udpport     string           //udp端口
	Listener    *net.UDPConn     //udp监听
	ToBroadCast []ServerImpl     //广播对象
	Serial      string           //服务号
	CryptInf    CryptImpl        //加密解密接口
	ParseInf    PackageParseImpl //消息解析
	DataQueue   chan DataWrapper //udp地址信息
	Pool        *MemPool         //pool
	UdpMap      sync.Map         //存放udp信息
}

func (u *UdpServerEntity) Init(port, serial string, cap_ int, pool *MemPool, cv CryptImpl, pv PackageParseImpl) {
	u.udpport = port
	u.Serial = serial
	u.CryptInf = cv
	u.ParseInf = pv
	u.ToBroadCast = make([]ServerImpl, 0, cap_)
	u.Pool = pool
	u.DataQueue = make(chan DataWrapper, 1000)
}

func (u *UdpServerEntity) AddToDistributeEntity(v ServerImpl) {
	u.ToBroadCast = append(u.ToBroadCast, v)
}

func (u *UdpServerEntity) SerialIsActivity(serial string) bool {
	return false
}

func (u *UdpServerEntity) GetSerial() string {
	return u.Serial
}

func (u *UdpServerEntity) AddDataForWrite(data DataWrapper) {
	u.DataQueue <- data
}

func (u *UdpServerEntity) BroadCastData(data DataWrapper) {
	for _, obj := range u.ToBroadCast {
		if obj != nil {
			obj.AddDataForWrite(data)
		}
	}
}

func (u *UdpServerEntity) readUdpData() {
	src := u.Pool.GetEntity(1, 1024)
	defer src.ReleaseOnece()
	serial := ""
	for {
		bytes, _ := src.Bytes()
		size, addr, err := u.Listener.ReadFromUDP(bytes)
		if err != nil {
			continue
		}
		toself := u.Pool.GetEntity(1, 1024)
		tocast := u.Pool.GetEntity(len(u.ToBroadCast), 1024)

		s_size, c_size, serial0, _ := u.ParseInf.Parser(u.Serial, serial, src, toself, tocast, &size, u.CryptInf)

		if s_size > 0 {
			s_write := DataWrapper{
				DataStore:       toself,
				UdpAddr:         addr,
				DataLength:      s_size,
				TargetConSerial: serial,
				CreateUnixSec:   time.Now().Unix(),
			}
			u.AddDataForWrite(s_write)
		} else {
			toself.FullRelease()
		}

		if c_size > 0 {
			c_write := DataWrapper{
				DataStore:       tocast,
				UdpAddr:         addr,
				DataLength:      c_size,
				TargetConSerial: serial0,
				CreateUnixSec:   time.Now().Unix(),
			}

			u.BroadCastData(c_write)
		} else {
			tocast.FullRelease()
		}

		if len(serial) > 1 {
			u.UdpMap.Store(serial, addr)
		}
	}
}

func (u *UdpServerEntity) writeUdpData() {
	var data DataWrapper
	for {
		data = <-u.DataQueue
		bytes, _ := data.DataStore.Bytes()
		if data.UdpAddr != nil {
			u.Listener.WriteToUDP(bytes[0:data.DataLength], data.UdpAddr)
		} else {
			if addr, ok := u.UdpMap.Load(data.TargetConSerial); ok {
				if udp, ok := addr.(*net.UDPAddr); ok {
					u.Listener.WriteToUDP(bytes[0:data.DataLength], udp)
				}
			}
		}
		data.DataStore.ReleaseOnece()
	}
}

func (u *UdpServerEntity) StartListen() {
	udpAddr, err := net.ResolveUDPAddr("udp", ":"+u.udpport)
	if err != nil {
		log.Fatal("Failed to gen udp addr")
	}
	u.Listener, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal("Failed to build udp listener")
	}
	for i := 0; i < 4; i++ {
		go u.writeUdpData()
	}
	u.readUdpData()
}
