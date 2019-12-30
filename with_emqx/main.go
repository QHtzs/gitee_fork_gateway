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
	if h.Last == "" { //针对第一次publish TTG/GATEWAY/xx 时，其它客户端无法订阅到消息做一次补充发送
		mqttclient.Publish("TTG/GATEWAY/"+con.Serial(), 0, false, str)
	}
	mqttclient.Publish("TTG/GATEWAY/"+con.Serial(), 0, false, str)
	h.Last = str
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
		con.SetReadDeadline(time.Time{})
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

//代理
type ServerProxy struct {
	client      mqtt.Client
	server      interaction.ServerImp
	server_type int
}

//初始化
func (s *ServerProxy) Init(client_id string, server_type int, sv interaction.ServerImp) {
	s.client = mqttclient.CreatClient(client_id)
	s.server_type = server_type
	s.server = sv
}

//消息发送到网关服务
func (s *ServerProxy) Pub(serial string, data []byte) {
	s.client.Publish("TTG/GATEWAY/"+serial, 0, false, data)
}

//鉴权并获取序列号
func (s *ServerProxy) Auth(con interaction.ConImp) bool {
	if s.server_type == 0 {
		con.SetReadDeadline(time.Now().Add(10 * time.Second))
		buf := make([]byte, 216, 216)
		length, err := con.Read(buf)
		if err != nil {
			return false
		}
		if length > 5 && buf[0] == 'T' && buf[1] == 'T' && buf[2] == 'G' {
			con.SetReadDeadline(time.Time{})
			con.SetSerial(string(buf[0:length]))
			con.Write([]byte(WEB_ACCEPT))
			return true
		}
		return false
	} else {
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
}

//添加监听回调
func (s *ServerProxy) SetCallback() {
	reg := regexp.MustCompile("[^/]+$")
	s.client.Subscribe("TTG/CONTROL/#", 0, func(c mqtt.Client, m mqtt.Message) {
		device_serial := reg.FindString(m.Topic())
		utils.Logger.Println("recv new info", m.Topic(), " server serial=", s.server.ClientSerial(), " serial=", device_serial)
		if len(device_serial) > 0 {
			payload := m.Payload()
			if len(payload) > 0 {
				if payload[0] <= 2 {
					url := fmt.Sprintf("%s?serial=%s&gateway_status=%i", utils.CfgInstance.StatusUrl, device_serial, m.Payload()[0])
					utils.HttpGet(url)
				} else {
					s.server.Publish(device_serial, payload)
				}
			}
		}
	})
}

//启动服务
func (s *ServerProxy) StartServer() {
	s.server.StartListen()
}

//所有服务共享一个mqtt client。并发实时性未充分利用,但是有利于消息统一管理
func RunServerWithOneMqttClient() {
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

	//add tcp service
	for _, v := range utils.CfgInstance.Tcp {
		tcp := &interaction.TcpServer{Serial: v.Serial, Port: v.Port}
		tcp_servers = append(tcp_servers, tcp)
		hd.AddServer(tcp)
		hd.AddRelation(v.Serial, v.Type)
	}

	//start services
	wss.Init(true, hd.Connected, hd.DeConnect, hd.WssPub, hd.WssAuth)
	for _, t_s := range tcp_servers {
		t_s.Init(true, hd.Connected, hd.DeConnect, hd.TcpPub, hd.TcpAuth)
		t_s.StartAuthRoutine(3)
		go t_s.StartListen()
	}
	wss.StartListen()
}

//每个服务拥有一个mqtt client.
func RunServerWithMqttClients() {

	hd := Handle{
		Servers:  make([]interaction.ServerImp, 0),
		Relation: make(map[string]string, 0),
		Buf:      make(map[string]bool, 0),
		Cas:      0,
		Last:     "",
	}

	tcp_servers := make([]*interaction.TcpServer, 0)
	proxys := make([]*ServerProxy, 0)

	wss_serial := "WEBSOCKET"
	wss := &interaction.WsServer{Serial: wss_serial}
	hd.AddServer(wss)
	hd.AddRelation(wss_serial, utils.CfgInstance.Wss.Type)

	ws_proxy := &ServerProxy{}
	wss.Init(true, hd.Connected, hd.DeConnect, ws_proxy.Pub, ws_proxy.Auth)
	ws_proxy.Init(wss_serial, 1, wss)

	for _, v := range utils.CfgInstance.Tcp {
		tcp := &interaction.TcpServer{Serial: v.Serial, Port: v.Port}
		tcp_servers = append(tcp_servers, tcp)
		hd.AddServer(tcp)
		hd.AddRelation(v.Serial, v.Type)

		tcp_proxy := &ServerProxy{}
		tcp.Init(true, hd.Connected, hd.DeConnect, tcp_proxy.Pub, tcp_proxy.Auth)
		tcp_proxy.Init(v.Serial, 0, tcp)
		tcp.StartAuthRoutine(3)

		proxys = append(proxys, tcp_proxy)
	}

	for _, v := range proxys {
		v.SetCallback()
		go v.StartServer()
	}

	ws_proxy.SetCallback()
	ws_proxy.StartServer()

}

func main() {
	RunServerWithMqttClients()
}
