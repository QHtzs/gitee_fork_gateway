package main

import (
	"cloudserver/with_emqx/interaction"
	"cloudserver/with_emqx/mqttclient"
	"cloudserver/with_emqx/utils"
	"fmt"
	"regexp"
	"sync/atomic"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
	"golang.org/x/net/websocket"
)

var (
	WEB_UPDATE  = `{"Gate_UPDATE": {"mode": "web" }}`
	APP_UPDATE  = `{"Gate_UPDATE": {"mode": "app" }}`
	ALL_UPDATE  = `{"Gate_UPDATE": {"mode": "all" }}`
	NONE_UPDATE = `{"Gate_UPDATE": {"mode": "none"}}`
	WEB_ACCEPT  = "WEB_ACCEPT"
)

//关系关联类
type Handle struct {
	Servers  []interaction.ServerImp
	Relation map[string]string
	Cas      int32
	Last     string
	Buf      map[string]bool
}

//添加 服务名:服务ID 映射关系
func (h *Handle) AddRelation(k, v string) {
	h.Relation[k] = v
}

//添加服务关系
func (h *Handle) AddServer(v interaction.ServerImp) {
	h.Servers = append(h.Servers, v)
}

//新建连接
func (h *Handle) Connected(t interaction.ServerImp, con interaction.ConImp) {
	for !atomic.CompareAndSwapInt32(&h.Cas, 0, 1) {
	} //自旋
	serial := con.Serial()

	for _, v := range h.Relation {
		h.Buf[v] = false
	}

	for _, v := range h.Servers {
		h.Buf[h.Relation[v.ClientSerial()]] = v.ClientIsOnline(serial) || h.Buf[h.Relation[v.ClientSerial()]]
	}

	var str string

	if h.Buf["WEB"] && h.Buf["APP"] {
		str = ALL_UPDATE
	} else if h.Buf["WEB"] {
		str = WEB_UPDATE
	} else if h.Buf["APP"] {
		str = APP_UPDATE
	} else {
		str = NONE_UPDATE
	}

	if str == h.Last {
		return
	}
	mqttclient.Publish("TTG/GATEWAY/"+con.Serial(), 1, false, str)
	atomic.StoreInt32(&h.Cas, 0)
}

//断开连接
func (h *Handle) DeConnect(t interaction.ServerImp, con interaction.ConImp) {
	h.Connected(t, con)
}

//TCP消息发送到网关服务
func (h *Handle) TcpPub(serial string, data []byte) {
	mqttclient.Publish("TTG/GATEWAY/"+serial, 0, false, data)
}

//wss消息发送到网关服务
func (h *Handle) WssPub(serial string, data []byte) {
	h.TcpPub(serial, data)
}

//Tcp鉴权并获取序列号
func (h *Handle) TcpAuth(con interaction.ConImp) bool {
	con.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 216, 216)
	length, err := con.Read(buf)
	if err != nil {
		return false
	}
	if length > 5 && buf[0] == 'T' && buf[1] == 'T' && buf[2] == 'G' {
		con.SetSerial(string(buf[0:length]))
		con.Write([]byte(WEB_ACCEPT))
		return true
	}
	return false
}

//Wss鉴权并获取序列号
func (h *Handle) WssAuth(con interaction.ConImp) bool {
	con.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 0, 0)
	w_con, ok := con.(*interaction.WCon)
	if !ok {
		return false
	}
	err := websocket.Message.Receive(w_con.Conn, &buf)
	if err != nil {
		return false
	}
	con.SetReadDeadline(time.Time{})
	if len(buf) > 5 && buf[0] == 'T' && buf[1] == 'T' && buf[2] == 'G' {
		con.SetSerial(string(buf[0:len(buf)]))
		websocket.Message.Send(w_con.Conn, WEB_ACCEPT)
		return true
	}
	return false

}

//网关消息转发到各个服务器
func (h *Handle) CreateRecieverHandle() {
	reg := regexp.MustCompile("[^/]+$")
	mqttclient.Subsribe("TTG/CONTROL/#", 0, func(c mqtt.Client, m mqtt.Message) {
		device_serial := reg.FindString(m.Topic())
		utils.Logger.Println("recv new info", m.Topic(), " serial=", device_serial)
		if len(device_serial) > 0 {
			payload := m.Payload()
			if len(payload) > 0 {
				if payload[0] <= 2 {
					url := fmt.Sprintf("%s?serial=%s&gateway_status=%i", utils.CfgInstance.StatusUrl, device_serial, m.Payload()[0])
					utils.HttpGet(url)
				} else {
					for _, sv := range h.Servers {
						sv.Publish(device_serial, payload)
					}
				}
			}
		}
	})
}

func main() {
	mqttclient.ReConnect()
	hd := Handle{
		Servers:  make([]interaction.ServerImp, 0),
		Relation: make(map[string]string, 0),
		Buf:      make(map[string]bool, 0),
		Cas:      0,
		Last:     "",
	}

	tcp_servers := make([]*interaction.TcpServer, 0)

	wss_serial := "WEBSOCKET"
	wss := &interaction.WsServer{Serial: wss_serial}
	hd.AddServer(wss)
	hd.AddRelation(wss_serial, utils.CfgInstance.Wss.Type)
	hd.CreateRecieverHandle()

	//start tcp service
	for _, v := range utils.CfgInstance.Tcp {
		tcp := &interaction.TcpServer{Serial: v.Serial, Port: v.Port}
		tcp_servers = append(tcp_servers, tcp)
		hd.AddServer(tcp)
		hd.AddRelation(v.Serial, v.Type)
	}

	//start wws service
	wss.Init(true, hd.Connected, hd.DeConnect, hd.WssPub, hd.WssAuth)
	for _, t_s := range tcp_servers {
		t_s.Init(true, hd.Connected, hd.DeConnect, hd.TcpPub, hd.TcpAuth)
		t_s.StartAuthRoutine(3)
		go t_s.StartListen()
	}
	wss.StartListen()
}
