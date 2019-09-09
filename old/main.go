// CloudServer project CloudServer.go
package main

import (
	"cloudserver/tcpservers"
	"cloudserver/utils"
	"fmt"
	"net"
	"strconv"
	"time"
)

var ALIVE []byte = []byte(utils.ConfigInstance.BeatPackages.GateWay)
var SLEN int = len(ALIVE)
var WEB_UPDATE []byte = []byte(`{"Gate_UPDATE": {"mode": "web"}}`)
var APP_UPDATE []byte = []byte(`{"Gate_UPDATE": {"mode": "app"}}`)
var ALL_UPDATE []byte = []byte(`{"Gate_UPDATE": {"mode": "all"}}`)
var NONE_UPDATE []byte = []byte(`{"Gate_UPDATE": {"mode": "none"}}`)
var WEB_ACCEPT []byte = []byte("WEB_ACCEPT")
var WEB_ACCEPT_ENCRY []byte = []byte("C0VVD0XFQ0QV=AV=")

//清除心跳包
func remove_beat(buf []byte, length, index int) {
	for i := index; i < length; i++ {
		if i < length-SLEN {
			buf[i] = buf[i+SLEN]
		} else {
			buf[i] = 0
		}
	}
}

//检测是否存在心跳包
func contains(buf []byte, length int) int {
	i := 0
	k := -1
	for i < length {
		if buf[i] == ALIVE[0] {
			for j := 1; j < SLEN; j++ {
				i += 1
				if buf[i] == ALIVE[j] {
					if j+1 == SLEN {
						k = i - j
						return k
					}
				} else {
					k = -1
					break
				}

			}
		} else {
			i += 1
		}

	}
	return k
}

//移除心跳包, 加密解密有些小问题
func ReMoveBeatBytes(buf []byte, length int) int {
	k := contains(buf, length)
	if k == -1 {
		return length
	}
	for k > -1 && length > 0 {
		remove_beat(buf, length, k)
		length -= 5
		k = contains(buf, length)
	}
	return length
}

//移除左端字符trimleft
func TrimLeft(data []byte, length int, ch byte) int {
	m := 0
	for i := 0; i < length; i++ {
		if data[i] == ch {
			m += 1
		} else {
			break
		}
	}
	if m == 0 {
		return length
	}
	for i := 0; i < length; i++ {
		data[i] = data[m+i]
	}
	return length - m
}

type BeatTool struct {
}

//生成心跳确认包
func (b *BeatTool) GenAckBeatBytes() []byte {
	return ALIVE
}

//tcp心跳时间(秒)
func (b *BeatTool) BeatFreqSec() int64 {
	return 50
}

//加密解密实例
type CryptInstance struct {
}

// =======================针对包加密,长度不够3的倍数时填充' '===============================================
//发送数据加密,块加密时，请注意池piece_size不宜过小,否则会溢出
func (c *CryptInstance) EncryPt(src, tmp []byte, src_len, tmp_len int) (int, bool) {
	if (src_len+2)/3*4 > tmp_len {
		return -1, false
	}
	l := utils.EncryPt(src, tmp, src_len, true) //补 ' ' (char=32)后，不会影响后续的json结构
	return l, true
}

//发送数据解密
func (c *CryptInstance) DeCrypt(src, tmp []byte, src_len, tmp_len int) (int, bool) {
	if src_len*3/4 > tmp_len {
		return -1, false
	}
	l := utils.DeCrypt(src, tmp, src_len)
	return l, true
}

// ====================================================================================================

type SerialTriggerBroadcast struct {
}

func (s *SerialTriggerBroadcast) DoConDistribute(serial string, v tcpservers.ServerImpl, hd tcpservers.PoolHandle, vs ...tcpservers.ServerImpl) {
	mp := make(map[string]bool, 1+len(vs))
	mp[v.GetSerial()] = v.SerialIsActivity(serial)
	for _, v0 := range vs {
		mp[v0.GetSerial()] = v0.SerialIsActivity(serial)
	}
	app, ok := mp["APP"]
	if !ok {
		app = false
	}
	web, ok := mp["WEB"]
	if !ok {
		web = false
	}
	var bts []byte
	if app && web {
		bts = ALL_UPDATE
	} else if app {
		bts = APP_UPDATE
	} else if web {
		bts = WEB_UPDATE
	} else {
		bts = NONE_UPDATE
	}
	hd1 := hd.BrrowAndAcq(len(mp))
	bytes := hd1.GetBytes()
	for i := 0; i < len(bts); i++ {
		bytes[i] = bts[i]
	}

	v.DistributeDataToWrite(hd1, len(bts), serial)

}

func (s *SerialTriggerBroadcast) DoDisDistribute(serial string, v tcpservers.ServerImpl, hd tcpservers.PoolHandle, vs ...tcpservers.ServerImpl) {
	mp := make(map[string]bool, 1+len(vs))
	mp[v.GetSerial()] = v.SerialIsActivity(serial)
	for _, v0 := range vs {
		mp[v0.GetSerial()] = v0.SerialIsActivity(serial)
	}

	app, ok := mp["APP"]
	if !ok {
		app = false
	}
	web, ok := mp["WEB"]
	if !ok {
		web = false
	}
	if !app && !web {
		hd1 := hd.BrrowAndAcq(len(mp))
		bytes := hd1.GetBytes()
		for i := 0; i < len(NONE_UPDATE); i++ {
			bytes[i] = NONE_UPDATE[i]
		}
		v.DistributeDataToWrite(hd1, len(NONE_UPDATE), serial)
	}
}

//更新web状态
func HttpSend(serial string, status int) error {
	url := fmt.Sprintf("%s?serial=%s&gateway_status=%i", utils.ConfigInstance.Other.StatusUrl, serial, status)
	err := utils.HttpMethod(url, "GET")
	utils.HSet("STATUS", serial, strconv.Itoa(status))
	return err
}

//消息id获取
func AckSerialDecry(con net.Conn, buf, buf1 []byte) (bool, string) {
	serial := ""
	status := false
	con.SetReadDeadline(time.Now().Add(time.Duration(utils.ConfigInstance.Other.TimeOut) * time.Second))
	size, err := con.Read(buf1)
	if err != nil {
		return status, serial
	}
	con.SetReadDeadline(time.Time{})

	if size > 8 && buf[0] == 'M' && buf[1] == 'O' && buf[2] == 'N' {
		serial = string(buf[0:size])
		status = true

	} else {

		size = utils.DeCrypt(buf1, buf, size)

		for i := size - 1; i >= 0; i-- {
			if buf[i] == ' ' || buf[i] == 0 {
				size -= 1
			} else {
				break
			}
		}
		if buf[0] == 'T' && buf[1] == 'T' && buf[2] == 'G' {
			serial = string(buf[0:size])
			status = true
		}

	}
	return status, serial
}

//消息id获取
func AckSerial(con net.Conn, buf, buf1 []byte) (bool, string) {
	serial := ""
	status := false

	con.SetReadDeadline(time.Now().Add(2 * time.Second))
	size, err := con.Read(buf)
	if err != nil {
		return status, serial
	}
	con.SetReadDeadline(time.Time{})

	for i := size - 1; i >= 0; i-- {
		if buf[i] == ' ' || buf[i] == 0 {
			size -= 1
		} else {
			break
		}
	}

	if buf[0] == 'T' && buf[1] == 'T' && buf[2] == 'G' {
		serial = string(buf[0:size])
		status = true
	}

	if buf[0] == 'M' && buf[1] == 'O' && buf[2] == 'N' {
		serial = string(buf[0:size])
		status = true
	}

	return status, serial
}

//回复确认消息
func Conform(con net.Conn, serial string) error {
	_, err := con.Write(WEB_ACCEPT_ENCRY)
	if err != nil {
		return err
	}
	err = HttpSend(serial, 1)
	return err
}

//回复确认消息
func Conform1(con net.Conn, serial string) error {
	_, err := con.Write(WEB_ACCEPT)
	return err
}

//断开回调
func Callback(serial string) {
	HttpSend(serial, -1)
}

//===================解析读取的数据=================================================

func findJsonTail(bys []byte, l int) int {
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

type WebParser struct {
}

func (w *WebParser) ParseRecvData(serial string, src, dst []byte, src_len *int) int {
	*src_len = ReMoveBeatBytes(src, *src_len)
	index := findJsonTail(src, *src_len)
	if index == -1 {
		return -1
	}
	for i := 0; i <= index; i++ {
		dst[i] = src[i]
	}
	*src_len = *src_len - index - 1
	for i := 0; i < *src_len; i++ {
		src[i] = src[index+1+i]
	}
	tl := TrimLeft(dst, index+1, ' ')
	return tl
}

type GateWayParse struct {
}

func (g *GateWayParse) ParseRecvData(serial string, src, dst []byte, src_len *int) int {
	*src_len = ReMoveBeatBytes(src, *src_len)
	index := findJsonTail(src, *src_len)
	if index == -1 {
		return -1
	}
	for i := 0; i <= index; i++ {
		dst[i] = src[i]
	}
	*src_len = *src_len - index - 1
	for i := 0; i < *src_len; i++ {
		src[i] = src[index+1+i]
	}
	tl := TrimLeft(dst, index+1, ' ')
	tl = TrimLeft(dst, tl, '#')
	return tl
}

type AppParse struct {
}

func (a *AppParse) ParseRecvData(serial string, src, dst []byte, src_len *int) int {
	*src_len = ReMoveBeatBytes(src, *src_len)
	index := findJsonTail(src, *src_len)
	if index == -1 {
		return -1
	}
	for i := 0; i <= index; i++ {
		dst[i] = src[i]
	}
	*src_len = *src_len - index - 1
	for i := 0; i < *src_len; i++ {
		src[i] = src[index+1+i]
	}
	tl := TrimLeft(dst, index+1, ' ')
	return tl
}

//===============================================================================
//程序入口
func main() {
	m := tcpservers.PoolManage{}
	m.Init()
	m.AddNew(1024, 2000)    //创建含2000个元素， 每个元素长为1024byte的内存池
	phd, err := m.Get(1024) //取出片段长为1024byte的池句柄
	if err != nil {
		panic("池出错")
	}
	BTool := BeatTool{}
	Server1 := tcpservers.TcpServerEntity{}              //web
	Server1.Init(1)                                      //数据只需要发送到一个对象(GATEWAY)
	Server1.SettingBeatObject(nil)                       //nil，则不发送心跳包
	Server1.SettingBroadCastType(false)                  //传输到web的数据不需要强制确定是否发送
	Server1.SettingDecryptObject(nil)                    //与客户端数据交互不需要加密
	Server1.SettingRecvParseObject(&WebParser{})         //添加解析逻辑
	Server1.SettingSerial("WEB")                         //服务标志
	Server1.SettingConTrigger(&SerialTriggerBroadcast{}) //
	Server1.SettingWriteType(3)                          //启用读线程池

	Server2 := tcpservers.TcpServerEntity{}              // app 即control
	Server2.Init(1)                                      //数据只需要发送到一个对象(GATEWAY)
	Server2.SettingBeatObject(nil)                       //nil，则不发送心跳包
	Server2.SettingBroadCastType(false)                  //传输到app的数据不需要强制确定是否发送
	Server2.SettingDecryptObject(nil)                    //与客户端数据交互不需要加密
	Server2.SettingRecvParseObject(&AppParse{})          //添加解析逻辑
	Server2.SettingSerial("APP")                         //服务标志
	Server2.SettingConTrigger(&SerialTriggerBroadcast{}) //
	Server2.SettingWriteType(3)                          //启用线程池

	Server3 := tcpservers.TcpServerEntity{}         // gateway
	Server3.Init(2)                                 //数据需要广播到两个对象（APP, WEB)
	Server3.SettingBeatObject(&BTool)               //nil，则不发送心跳包
	Server3.SettingBroadCastType(true)              //传输到gateway的数据需要强制确定是否发送
	Server3.SettingDecryptObject(&CryptInstance{})  //与客户端数据交互需要加密
	Server3.SettingRecvParseObject(&GateWayParse{}) //添加解析逻辑
	Server3.SettingSerial("GATEWAY")                //服务标志
	Server2.SettingConTrigger(nil)                  //
	Server3.SettingWriteType(0)                     //不启用线程池,每个socket一个微线程(goroutin)

	Server1.AddToDistribute(&Server3) //添加广播对象 相互引用问题,三个对象都不被引用才会都被回收
	Server2.AddToDistribute(&Server3) //添加广播对象
	Server3.AddToDistribute(&Server1) //添加广播对象
	Server3.AddToDistribute(&Server2) //添加广播对象
	//HttpSend("", -1)
	go Server1.StartServer(utils.ConfigInstance.TcpPorts.WebClient, 600, &phd, AckSerial, Conform1, nil)     //取goroutin跑
	go Server2.StartServer(utils.ConfigInstance.TcpPorts.Control, 600, &phd, AckSerial, Conform1, nil)       //取goroutin跑
	Server3.StartServer(utils.ConfigInstance.TcpPorts.GateWay, 600, &phd, AckSerialDecry, Conform, Callback) //阻塞

}
