package main

/*
网关转发系统
@author: TTG
@brief: 绝大部分具体逻辑实现实例
*/

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"
)

const (
	GATEWAY_CONNECT    = 1
	GATEWAY_DISCONNECT = -1
	GATEWAY_UNKNOWN    = 0
	SERVER_WEB         = "WEB"
	SERVER_APP         = "APP"
	SERVER_GATEWAY     = "GATEWAY"
	WEBSOCKET          = "WEBSOCKET"
)

var ALIVE []byte = []byte(ConfigInstance.BeatPackages.GateWay)
var SLEN int = len(ALIVE)
var WEB_UPDATE []byte = []byte(`{"Gate_UPDATE": {"mode": "web" }}`)
var APP_UPDATE []byte = []byte(`{"Gate_UPDATE": {"mode": "app" }}`)
var ALL_UPDATE []byte = []byte(`{"Gate_UPDATE": {"mode": "all" }}`)
var NONE_UPDATE []byte = []byte(`{"Gate_UPDATE": {"mode": "none"}}`)
var WEB_ACCEPT []byte = []byte("WEB_ACCEPT")

//心跳实例化
type BeatPackageEntity struct {
	Beat     []byte
	AckBeat  []byte
	Interval int64 //为0时则只被动响应，不发送心跳, 单位秒
}

//生成心跳内容
func (b *BeatPackageEntity) BeatBytes() []byte {
	return b.Beat
}

//生成心跳响应内容
func (b *BeatPackageEntity) BeatAckBytes() []byte {
	return b.AckBeat
}

//心跳发送时间间隔
func (b *BeatPackageEntity) BeatInteval() int64 {
	return b.Interval
}

//加密解密实例化
type CryptEntity struct {
}

//判断 bytes是否被包含
func contains(src, substr []byte, src_len, substr_len int) (bool, int) {
	ret := false

	if substr_len > src_len {
		return ret, 0
	}

	for i := 0; i < src_len-substr_len+1; i++ {
		if src[i] == substr[0] {
			ret = true
			for j := 1; j < substr_len && ret; j++ {
				ret = src[i+j] == substr[j]
			}
			if ret {
				return ret, i
			}
		}
	}
	return ret, 0
}

//从二进制字符中移除字节
func removeBytes(src []byte, index, src_size, sub_size int) {
	for i := index; i < src_size; i++ {
		if i < src_size-sub_size {
			src[i] = src[i+sub_size]
		}
	}
}

//清除左边空白
func trimLeft(src []byte, src_len int, ch byte) int {
	m := 0
	for i := 0; i < src_len; i++ {
		if src[i] == ch {
			m += 1
		} else {
			break
		}
	}
	removeBytes(src, 0, src_len, m)
	return src_len - m

}

//加密函数
func (c *CryptEntity) EncryPt(src, dst []byte, src_len, dst_buff_len int) (bool, int, int) {

	k := src_len % 3

	if k > 0 {
		k = 3 - k
		for i := 0; i < k; i++ {
			src[src_len+i] = 32
		}
		src_len += k
	}

	if src_len*4/3 > dst_buff_len { //dst buff not enough
		return false, 0, 0
	}

	dlen := EncryPt(src, dst, src_len)

	return true, src_len, dlen

}

//解密函数
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

//tcp socket新连接接入触发信号, 消息传入对于gateway
func (c *ConChangeObserverEntity) SNewConnect(serial string, entity *MemEntity, v ServerImpl, vs ...ServerImpl) {

	mp := v.SerialActivityMap(serial)
	for _, vi := range vs {
		mp0 := vi.SerialActivityMap(serial)
		for key, value := range mp0 {
			mp[key] = value
		}
	}

	app := mp[SERVER_APP]
	web := mp[SERVER_WEB]

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
		SelfId:          "",
		CreateUnixSec:   time.Now().Unix(),
	}

	//推送给GATEWAY
	if v.GetSerial() == SERVER_GATEWAY {
		time.Sleep(200 * time.Millisecond)
		v.AddDataForWrite(data)
	} else {
		for _, s := range vs {
			if s.GetSerial() == SERVER_GATEWAY {
				s.AddDataForWrite(data)
			}
		}
	}
}

//tcp socket断开触发 消息传入对于gateway
func (c *ConChangeObserverEntity) SDisConnect(serial string, entity *MemEntity, v ServerImpl, vs ...ServerImpl) {
	if v.GetSerial() == SERVER_GATEWAY {
		return
	}
	c.SNewConnect(serial, entity, v, vs...)
}

//tcp socket新连接接入触发信号, 消息传入http服务
func (c *ConChangeObserverEntity) HNewConnect(serial string, v ServerImpl) {
	if v.GetSerial() == SERVER_GATEWAY { //当对象为GATEWAY触发
		url := fmt.Sprintf("%s?serial=%s&gateway_status=%i", ConfigInstance.Other.StatusUrl, serial, GATEWAY_CONNECT)
		HttpGet(url)
	}
	//HSet("STATUS", serial, "1") //不采用redis记录状态
}

//tcp socket断开触发, 消息传入http服务
func (c *ConChangeObserverEntity) HDisConnect(serial string, v ServerImpl) {
	if v.GetSerial() == SERVER_GATEWAY { //当对象为GATEWAY触发
		url := fmt.Sprintf("%s?serial=%s&gateway_status=%i", ConfigInstance.Other.StatusUrl, serial, GATEWAY_DISCONNECT)
		HttpGet(url)
	}
	//HSet("STATUS", serial, "0") //不采用redis记录状态
}

//认证实例化
type AckEntity struct {
}

//新连接时认证， 含monitor查看端认证，和设备连接认证
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

	for size > 8 {
		if data[size-1] == 32 || data[size-1] == 0 {
			size -= 1
		} else {
			break
		}
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

//接收或者拒绝连接
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
			//bytes = WEB_ACCEPT_ENCRY // just for test, remove this line in release version
			//d = len(bytes)           // just for test, remove this line in release version
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

//简单规则评估包是不是json。
func (p *TcpPackageParseEntity) checkMayJson(bys []byte, l int) (from int, to int) {
	from = -1
	to = -1
	if l <= 0 {
		return from, to
	}
	num := 0
	for i := 0; i < l; i++ {
		if bys[i] == '{' {
			if from == -1 {
				from = i
			}
			num += 1
		}
		if bys[i] == '}' {
			to = i
			num -= 1
		}

		if from >= 0 && to > from && num == 0 {
			break
		}
	}
	if from > to {
		from = -1
	}
	return from, to

}

//解析接收到的数据
func (p *TcpPackageParseEntity) Parser(server_serial, tcp_serial string, src, toself, tocast *MemEntity, src_len *int, v CryptImpl) (int, int, string, bool) {

	bytes, piece := src.Bytes()

	beat := false
	s_size := 0
	ca_size := 0
	serial := tcp_serial
	left_len := 0

	if *src_len == 0 {
		return 0, 0, serial, beat
	}

	entity0 := src.PoolPtr.GetEntity(1, piece)
	entity1 := src.PoolPtr.GetEntity(1, piece)
	defer entity0.FullRelease()
	defer entity1.FullRelease()

	cwp, _ := entity0.Bytes()
	cdt, _ := entity1.Bytes()
	dlen := *src_len
	ok := false

	if v == nil {
		for i := 0; i < dlen; i++ {
			cdt[i] = bytes[i]
		}
	} else {
		for i := 0; i < dlen; i++ {
			cwp[i] = bytes[i]
		}
	}

	if v != nil {
		ok, left_len, dlen = v.DeCrypt(cwp, cdt, dlen, piece)
		dlen = trimLeft(cdt, dlen, 32)
		if !ok {
			log.Println("解密失败", server_serial)
			return 0, 0, serial, beat
		}
	}
	ok, bi := contains(cdt, ALIVE, dlen, len(ALIVE))
	for ok {
		beat = true
		removeBytes(cdt, bi, dlen, len(ALIVE))
		dlen -= len(ALIVE)
		ok, bi = contains(cdt, ALIVE, dlen, len(ALIVE))
	}

	from, to := p.checkMayJson(cdt, dlen)

	if from > -1 {
		ca_size = to - from + 1
		tbyte, _ := tocast.Bytes()
		for i := from; i <= to; i++ {
			tbyte[i-from] = cdt[i]
		}
		for i := from; i <= to; i++ {
			cdt[i] = cdt[i+ca_size]
		}
		dlen -= ca_size
	}

	if v == nil {
		for i := 0; i < dlen; i++ {
			bytes[i] = cdt[i]
		}
	} else {
		if dlen > 0 {
			_, _, dlen = v.EncryPt(cdt, bytes, dlen, piece)
		}
		for i := 0; i < left_len; i++ {
			bytes[dlen+i] = cwp[i]
		}
		dlen += left_len
	}

	*src_len = dlen
	return s_size, ca_size, serial, beat

}

//Udp结构
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

//websocket信息解析
type WebSocketParse struct {
}

//websocket解析
func (w *WebSocketParse) Parser(server_serial, cur_con_serial string, src, toself, tocast *MemEntity, src_len *int, v CryptImpl) (s_size int, c_size int, serial string, beat bool) {
	bytes, _ := src.Bytes()
	//sbytes, _ := toself.Bytes()
	cbytes, _ := tocast.Bytes()
	for i := 0; i < *src_len; i++ {
		cbytes[i] = bytes[i]
		//sbytes[i] = bytes[i]
	}
	//sbytes[*src_len] = ':'
	//sbytes[*src_len+1] = 'O'
	//sbytes[*src_len+2] = 'K'
	return 0, *src_len, cur_con_serial, false
}

func main() {
	/*
		Tcp, WebSocket目前每个服务中所有的serial均不能雷同，否则会被挤下线
	*/

	pool := MemPool{}

	WEIXIN := UdpServerEntity{}
	WEIXIN.Init(ConfigInstance.Ports.UdpPort, "微信", 1, &pool, nil, &WEIXINPackageParseEntity{})

	GateWay := TcpServerEntity{}
	v := &BeatPackageEntity{
		Beat:     ALIVE,
		AckBeat:  ALIVE,
		Interval: 50,
	}

	if ConfigInstance.NeedEncrypt.GateWay {
		GateWay.Init(ConfigInstance.Ports.GateWay, SERVER_GATEWAY, false, true, 15000, 0, 3, 600,
			&pool,
			v,
			&CryptEntity{},
			&ConChangeObserverEntity{},
			&AckEntity{},
			&TcpPackageParseEntity{})
	} else {
		GateWay.Init(ConfigInstance.Ports.GateWay, SERVER_GATEWAY, false, true, 15000, 0, 3, 600,
			&pool,
			v,
			nil,
			&ConChangeObserverEntity{},
			&AckEntity{},
			&TcpPackageParseEntity{})
	}

	WEB := TcpServerEntity{}
	WEB.Init(ConfigInstance.Ports.WebClient, SERVER_WEB, true, false, 15000, 5, 1, 600,
		&pool,
		nil,
		nil,
		&ConChangeObserverEntity{},
		&AckEntity{},
		&TcpPackageParseEntity{})

	APP := TcpServerEntity{}
	APP.Init(ConfigInstance.Ports.Control, SERVER_APP, true, false, 15000, 5, 1, 600,
		&pool,
		nil,
		nil,
		&ConChangeObserverEntity{},
		&AckEntity{},
		&TcpPackageParseEntity{})

	WebSocket := WebSocketServerEntity{}
	WebSocket.Init(ConfigInstance.Ports.WsPort,
		WEBSOCKET,
		true,
		1,
		600,
		&pool,
		&WebSocketParse{})

	WEIXIN.AddToDistributeEntity(&GateWay)
	WEB.AddToDistributeEntity(&GateWay)
	APP.AddToDistributeEntity(&GateWay)
	WebSocket.AddToDistributeEntity(&GateWay)

	GateWay.AddToDistributeEntity(&WEB)
	GateWay.AddToDistributeEntity(&APP)
	GateWay.AddToDistributeEntity(&WebSocket)

	go WEB.StartListen()
	go APP.StartListen()
	go WEIXIN.StartListen()
	go GateWay.StartListen()

	if ConfigInstance.PProfDebuger {
		go WebSocket.StartListen()
		http.ListenAndServe("localhost:7777", nil) //block
	} else {
		WebSocket.StartListen() // block
	}
}

/*
断线快速连接后， （断开，连接）状态会发两次，由于异步操作，断开被触发后，由于快速连接，当执行获取状态时，连接为非断开，故连续发两次一样的数据
*/
