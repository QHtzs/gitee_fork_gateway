package main

/*
网关转发系统
*/

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"time"
)

const (
	GATEWAY_CONNECT    = 1
	GATEWAY_DISCONNECT = -1
	GATEWAY_UNKNOWN    = 0
	SERVER_WEB         = "WEB"
	SERVER_APP         = "APP"
	SERVER_GATEWAY     = "GATEWAY"
)

var ALIVE []byte = []byte(ConfigInstance.BeatPackages.GateWay)
var SLEN int = len(ALIVE)
var WEB_UPDATE []byte = []byte(`{"Gate_UPDATE": {"mode": "web"}}`)
var APP_UPDATE []byte = []byte(`{"Gate_UPDATE": {"mode": "app"}}`)
var ALL_UPDATE []byte = []byte(`{"Gate_UPDATE": {"mode": "all"}}`)
var NONE_UPDATE []byte = []byte(`{"Gate_UPDATE": {"mode": "none"}}`)
var WEB_ACCEPT []byte = []byte("WEB_ACCEPT")
var WEB_ACCEPT_ENCRY []byte = []byte("C0VVD0XFQ0QV=AV=")

//心跳实例化
type BeatPackageEntity struct {
	Beat     []byte
	AckBeat  []byte
	Interval int64 //为0时则只被动响应，不发送心跳
}

func (b *BeatPackageEntity) BeatBytes() []byte {
	return b.Beat
}
func (b *BeatPackageEntity) BeatAckBytes() []byte {
	return b.AckBeat
}

func (b *BeatPackageEntity) BeatInteval() int64 {
	return b.Interval
}

//加密解密实例化
type CryptEntity struct {
}

func contains(src, substr []byte, src_len, substr_len int) (bool, int) {
	ret := false
	i := 0
	k := 0
	if substr_len > src_len {
		return ret, k
	}
	for i <= src_len-substr_len {
		if src[i] == substr[0] {
			for j := 1; j < substr_len; j++ {
				i += 1
				if src[i] == substr[i] {
					if j+1 == substr_len {
						k = i - j
						ret = true
						return ret, k
					}
				} else {
					break
				}
			}
		} else {
			i += 1
		}
	}
	return ret, k
}

func removeBeat(src []byte, index, src_size, sub_size int) {
	for i := index; i < src_size; i++ {
		if i < src_size-sub_size {
			src[i] = src[i+sub_size]
		}
	}
}

func trim(src []byte, src_len int, ch byte) int {
	m := 0
	for i := 0; i < src_len; i++ {
		if src[i] == ch {
			m += 1
		} else {
			break
		}
	}
	removeBeat(src, 0, src_len, m)

	for i := src_len - 1; i >= 0; i-- {
		if src[i] == ch {
			m += 1
		} else {
			break
		}
	}
	return src_len - m

}

func (c *CryptEntity) EncryPt(src, dst []byte, src_len, dst_buff_len int) (bool, int, int) {
	var buf []byte = nil
	var dbuf []byte = nil

	if (src_len+2)*4/3 > dst_buff_len { //dst buff not enough
		return false, 0, 0
	}

	k := src_len % 3

	if k > 0 {
		src_len -= k
		buf = make([]byte, 3, 3)
		dbuf = make([]byte, 4, 4)
		for i := 0; i < 3; i++ {
			if i < k {
				buf[i] = src[src_len+i]
			} else {
				buf[i] = ' '
			}
		}
	}

	dlen := EncryPt(src, dst, src_len)

	if k > 0 {
		tmp := EncryPt(buf, dbuf, 3)
		for i := 0; i < tmp; i++ {
			dst[dlen+i] = dbuf[i]
		}
		dlen += tmp
	}

	return true, src_len, dlen

}

func (c *CryptEntity) DeCrypt(src, dst []byte, src_len, dst_buff_len int) (bool, int, int) {
	if src_len*3/4 > dst_buff_len { //dst buff not enough
		return false, 0, 0
	}

	k := src_len % 4
	src_len -= k

	dlen := DeCrypt(src, dst, src_len)
	for i := 0; i < k; i++ {
		src[i] = src[i+src_len]
	}
	return true, k, dlen
}

//连接状态观察者实例化
type ConChangeObserverEntity struct {
}

func (c *ConChangeObserverEntity) SNewConnect(serial string, entity *MemEntity, v ServerImpl, vs ...ServerImpl) {
	web := false
	app := false
	for _, s := range vs {
		if s.SerialIsActivity(serial) {
			if s.GetSerial() == SERVER_WEB {
				web = true
			} else if s.GetSerial() == SERVER_APP {
				app = true
			}
		}
	}
	var bytes []byte
	if web && app {
		bytes = ALL_UPDATE
	} else if web {
		bytes = WEB_UPDATE
	} else if app {
		bytes = APP_UPDATE
	} else {
		bytes = NONE_UPDATE
	}

	_, piece := entity.Bytes()
	e := entity.PoolPtr.GetEntity(1, piece)
	buf, _ := e.Bytes()
	size := len(bytes)
	for i := 0; i < size; i++ {
		buf[i] = bytes[i]
	}
	data := DataWrapper{
		DataStore:       e,
		UdpAddr:         nil,
		DataLength:      size,
		TargetConSerial: serial,
		CreateUnixSec:   time.Now().Unix(),
	}
	v.AddDataForWrite(data)
}

func (c *ConChangeObserverEntity) SDisConnect(serial string, entity *MemEntity, v ServerImpl, vs ...ServerImpl) {
	c.SNewConnect(serial, entity, v, vs...)
}

func (c *ConChangeObserverEntity) HNewConnect(serial string) {
	url := fmt.Sprintf("%s?serial=%s&gateway_status=%i", ConfigInstance.Other.StatusUrl, serial, GATEWAY_CONNECT)
	HttpGet(url)
	HSet("STATUS", serial, "1")
}

func (c *ConChangeObserverEntity) HDisConnect(serial string) {
	url := fmt.Sprintf("%s?serial=%s&gateway_status=%i", ConfigInstance.Other.StatusUrl, serial, GATEWAY_DISCONNECT)
	HttpGet(url)
	HSet("STATUS", serial, "0")
}

//认证实例化
type AckEntity struct {
}

func (a *AckEntity) AckSerial(con net.Conn, buf *MemEntity, v CryptImpl) (bool, string, int) {
	data, piece := buf.Bytes()
	con.SetDeadline(time.Now().Add(5 * time.Second))
	size, err := con.Read(data)
	if err != nil {
		return false, "", 0
	}
	con.SetDeadline(time.Time{})

	if v != nil {
		tmp := buf.PoolPtr.GetEntity(1, piece)
		defer tmp.FullRelease()
		bytes, p0 := tmp.Bytes()
		status, _, d := v.DeCrypt(data, bytes, size, p0)
		if !status {
			return false, "", 0
		}

		size = d
		for i := 0; i < d; i++ {
			data[i] = bytes[i]
		}

	}

	if size < 8 {
		return false, "", 0
	}

	if data[0] == 'M' && data[1] == 'O' && data[2] == 'N' {
		//MONITOR_
		serial := string(data[8:size])
		return true, serial, 1

	} else if data[0] == 'T' && data[1] == 'T' && data[2] == 'G' {
		serial := string(data[0:size])
		return true, serial, 0

	} else {
		return false, "", 0
	}

}

func (a *AckEntity) AckConnect(con net.Conn, serial string, buf *MemEntity, v CryptImpl) error {
	var err error = nil
	if v == nil {
		_, err = con.Write(WEB_ACCEPT)

	} else {
		data, piece := buf.Bytes()
		size := len(WEB_ACCEPT)
		for i := 0; i < size; i++ {
			data[i] = WEB_ACCEPT[i]
		}

		tmp := buf.PoolPtr.GetEntity(1, piece)
		defer tmp.FullRelease()
		bytes, p0 := tmp.Bytes()
		status, _, d := v.EncryPt(data, bytes, size, p0)
		if status {
			bytes = WEB_ACCEPT_ENCRY // just for test, remove this line in release version
			d = len(bytes)           // just for test, remove this line in release version
			_, err = con.Write(bytes[0:d])
		} else {
			err = errors.New("加密失败")
		}
	}
	return err

}

//解包器实例化
type TcpPackageParseEntity struct {
}

//两个 }}结尾则认为合法
func (p *TcpPackageParseEntity) checkValidTail(bys []byte, l int) int {
	num := 0
	for i := 0; i < l; i++ {
		if bys[i] == '}' {
			num += 1
			if num == 2 {
				return i
			}
		} else if !(bys[i] == ' ' || bys[i] == '\t' || bys[i] == '\n' || bys[i] == '\r') {
			num = 0
		}
	}
	return -1
}

func (p *TcpPackageParseEntity) Parser(server_serial, tcp_serial string, src, toself, tocast *MemEntity, src_len *int, v CryptImpl) (int, int, string, bool) {
	src_str, piece := src.Bytes()
	beat := false
	s_size := 0
	ca_size := 0
	serial := tcp_serial

	entity0 := src.PoolPtr.GetEntity(1, piece)
	entity1 := src.PoolPtr.GetEntity(1, piece)
	defer entity0.FullRelease()
	defer entity1.FullRelease()

	bytes, _ := entity0.Bytes()
	dst_str, _ := entity1.Bytes()
	dlen := *src_len

	for i := 0; i < dlen; i++ {
		bytes[i] = src_str[i]
	}

	if v != nil {

		ok, k, dlen := v.DeCrypt(bytes, dst_str, dlen, piece)
		if !ok {
			log.Println("解密失败", server_serial)
			return 0, 0, serial, beat
		}

		ok, bi := contains(dst_str, ALIVE, dlen, len(ALIVE))

		for ok {
			beat = true
			removeBeat(dst_str, bi, dlen, len(ALIVE))
			dlen -= len(ALIVE)
			ok, bi = contains(dst_str, ALIVE, dlen, len(ALIVE))
		}

		index := p.checkValidTail(dst_str, dlen)
		if index > -1 {
			ca_size = index + 1
			tbyte, _ := tocast.Bytes()
			for i := 0; i < ca_size; i++ {
				tbyte[i] = dst_str[i]
			}

			fmt.Println(string(tbyte[0:ca_size]))

			for i := 0; i < dlen-ca_size; i++ {
				dst_str[i] = dst_str[i+ca_size]
			}

			dlen -= ca_size
		}

		if ca_size > 0 || ok {
			_, _, elen := v.EncryPt(dst_str, src_str, dlen, piece)
			for i := 0; i < k; i++ {
				src_str[elen+i] = bytes[i]
			}

			dlen = elen + k
		}

	} else {

		ok, bi := contains(bytes, ALIVE, dlen, len(ALIVE))

		for ok {
			beat = true
			removeBeat(bytes, bi, dlen, len(ALIVE))
			dlen -= len(ALIVE)
			ok, bi = contains(bytes, ALIVE, dlen, len(ALIVE))
		}

		index := p.checkValidTail(bytes, dlen)

		if index > -1 {
			ca_size = index + 1
			tbyte, _ := tocast.Bytes()
			for i := 0; i < ca_size; i++ {
				tbyte[i] = bytes[i]
			}

			fmt.Println(string(tbyte[0:ca_size]))

			for j := 0; j < dlen-ca_size; j++ {
				bytes[j] = bytes[j+ca_size]
			}

			dlen -= ca_size
		}

		if ok || index > -1 {
			for i := 0; i < dlen; i++ {
				src_str[i] = bytes[i]
			}
		}

	}
	*src_len = dlen
	return s_size, ca_size, serial, beat

}

//微信包结构
type UdpPackage struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Serial  string `json:"serial"`
	From    string `json:"from"`
}

func (u *UdpPackage) Deseries(js []byte) bool {
	err := json.Unmarshal(js, u)
	if err != nil {
		log.Println("udp包解析失败")
	}
	return err == nil
}

//微信包解析
type WEIXINPackageParseEntity struct {
}

//微信接口包解析逻辑。 待添加
func (w *WEIXINPackageParseEntity) Parser(server_serial, udp_serial string, src, toself, tocast *MemEntity, src_len *int, v CryptImpl) (int, int, string, bool) {
	bytes, _ := src.Bytes()
	p := UdpPackage{}
	if p.Deseries(bytes[0:*src_len]) {
		tcp_serial := p.Serial
		tc, _ := tocast.Bytes()
		for i := 0; i < *src_len; i++ {
			tc[i] = bytes[i]
		}
		return 0, *src_len, tcp_serial, false
	} else {
		return 0, 0, "", false
	}

}

func main() {

	pool := MemPool{}

	WEIXIN := UdpServerEntity{}
	WEIXIN.Init(ConfigInstance.TcpPorts.UdpPort, "微信", 1, &pool, nil, &WEIXINPackageParseEntity{})

	GateWay := TcpServerEntity{}
	v := &BeatPackageEntity{
		Beat:     ALIVE,
		AckBeat:  ALIVE,
		Interval: 50,
	}

	GateWay.Init(ConfigInstance.TcpPorts.GateWay, SERVER_GATEWAY, true, 0, 2, 3600, &pool, v,
		&CryptEntity{},
		&ConChangeObserverEntity{},
		&AckEntity{},
		&TcpPackageParseEntity{})

	WEB := TcpServerEntity{}
	WEB.Init(ConfigInstance.TcpPorts.WebClient, SERVER_WEB, false, 5, 1, 3600, &pool, nil, nil, nil,
		&AckEntity{},
		&TcpPackageParseEntity{})

	APP := TcpServerEntity{}
	APP.Init(ConfigInstance.TcpPorts.Control, SERVER_APP, false, 5, 1, 3600, &pool, nil, nil, nil,
		&AckEntity{},
		&TcpPackageParseEntity{})

	WEIXIN.AddToDistributeEntity(&GateWay)
	WEB.AddToDistributeEntity(&GateWay)
	APP.AddToDistributeEntity(&GateWay)
	GateWay.AddToDistributeEntity(&WEB)
	GateWay.AddToDistributeEntity(&APP)

	go WEB.StartListen()
	go APP.StartListen()
	go WEIXIN.StartListen()
	GateWay.StartListen()

}
