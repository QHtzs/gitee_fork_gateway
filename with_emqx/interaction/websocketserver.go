package interaction

/*
web socket
*/

import (
	"cloudserver/with_emqx/utils"

	"fmt"
	"net/http"

	"golang.org/x/net/websocket"
)

//index 页的html
var IndexPageBuff []byte = []byte(fmt.Sprintf(`
<html>
<head>
    <meta charset="utf-8"/>
    <title>Websocket</title>
</head>
<body>
    <h1>测试WebSocket</h1>
    <form>
        <p>
            发送信息: <input id="content" type="text" placeholder="TTG_DEMO_0001" value="TTG_DEMO_0001" />
        </p>
    </form>
    <label id="result">接收信息：</label><br><br>
    <input type="button" name="button" id="button" value="提交" onclick="sk_send()"/>
    <script type="text/javascript">
        var sock = null;
        var wsuri = "%s/%s"
        wsuri = wsuri.replace(/^wss*/g , document.location.protocol=="http:"? "ws":"wss")    
        sock = new WebSocket(wsuri);
        
        sock.onmessage = function(e) {
            var result = document.getElementById('result');
            console.log(e.data)
            console.log(e.data.text && e.data.text())
            var text = (e.data.text && e.data.text()) || e.data;
            if(text.text){text = "BLOG, NEED FILEREADER TO DISERIES";}
            result.innerHTML = "收到回复：" + text;
        }
        
        sock.onclose = function(e){
          alert("websocket 连接断开， 请刷新页面")
        }
            
        function sk_send() {
        	document.getElementById('result').innerHTML="收到回复:"
            var msg = document.getElementById('content').value;
            sock.send(msg);
        }
    </script>
</body>
</html>`, utils.CfgInstance.Wss.Domain, utils.CfgInstance.Wss.Path))

//ConImp接口针对websocket形象化
type WCon struct {
	serial string
	Isok   bool
	Peer   string
	*websocket.Conn
}

func (t *WCon) SetSerial(serial string) {
	t.serial = serial
}

func (t *WCon) Serial() string {
	return t.serial
}

func (t *WCon) Ok() bool {
	return t.Isok
}

func (t *WCon) Close() error {
	t.Isok = false
	return t.Conn.Close()
}

func (t *WCon) PeerAddress() string {
	return t.Peer
}

//ServerImp接口针对 websocket服务形象化
type WsServer struct {
	Cons           utils.NetConMap
	Serial         string
	ReadAndPublish func(serial string, data []byte)
	NewConCallBack func(t ServerImp, con ConImp)
	DisConCallBack func(t ServerImp, con ConImp)
	AuthFunc       func(con ConImp) bool
}

func (w *WsServer) ClientSerial() string {
	return w.Serial
}

func (w *WsServer) ClientIsOnline(serial string) bool {
	return w.Cons.IsKeyExist(serial)
}

func (w *WsServer) Publish(serial string, data []byte) {
	cons := w.LoadCons(serial)
	for _, v := range cons {
		ws, ok := v.(*WCon)
		if ok {
			websocket.Message.Send(ws.Conn, string(data))
		}
	}
}

//初始化
func (w *WsServer) Init(allow_dup bool, ncbk, dcbk func(t ServerImp, con ConImp), pub func(serial string, data []byte), authandsetserial func(con ConImp) bool) {
	w.NewConCallBack = ncbk
	w.DisConCallBack = dcbk
	w.ReadAndPublish = pub
	w.AuthFunc = authandsetserial
	w.Cons.SetAllowDup(allow_dup)
}

//断开连接，并触发DisConCallBack
func (w *WsServer) CloseCon(con ConImp) {
	con.Close()
	w.Cons.Delete(con.Serial(), con.RemoteAddr().String())
	if w.DisConCallBack != nil {
		w.DisConCallBack(w, con)
	}
}

//建立连接， 并触发NewConCallBack
func (w *WsServer) AddCon(con ConImp) bool {
	_, ok := w.Cons.LoadOrStore(con.Serial(), con.RemoteAddr().String(), con)
	if !ok && w.NewConCallBack != nil {
		w.NewConCallBack(w, con)
	}
	return !ok
}

//获取同 主机序列号下的所有上位机控制连接 [主机:控制 == 1:n]
func (w *WsServer) LoadCons(serial string) []ConImp {
	cons := make([]ConImp, 0, 1)
	w.Cons.Range(serial, func(k, v interface{}) bool {
		cn := v.(ConImp)
		cons = append(cons, cn)
		return true
	})
	return cons
}

//websocket具体业务逻辑实现, 用于绑定在http/https下
func (w *WsServer) WebSockHandle(ws *websocket.Conn) {
	defer ws.Close()
	buf := make([]byte, 0, 1024)
	req_addr := ws.Request().RemoteAddr
	var con *WCon = &WCon{Isok: true, Peer: req_addr, Conn: ws}

	if !w.AuthFunc(con) {
		return
	}

	serial := con.Serial()
	defer w.CloseCon(con)
	ok := w.AddCon(con)

	if !ok {
		utils.Logger.Println(w.Serial, serial, "con exist connect")
		return
	}
	utils.Logger.Println(w.Serial, serial, "connect")
	var err error
	var length int
	for {
		err = websocket.Message.Receive(ws, &buf)
		length = len(buf)
		if length == -1 || err != nil {
			utils.Logger.Println(w.Serial, serial, "dis connect")
			break
		}
		w.ReadAndPublish(serial, buf[0:length])
		websocket.Message.Send(ws, "ACK")
	}

}

//http服务，用于加载页面
func (w *WsServer) Index(hw http.ResponseWriter, r *http.Request) {
	hw.Write(IndexPageBuff)
}

//开启websocket服务。 依托http/https服务
func (w *WsServer) StartListen() {
	http.Handle("/"+utils.CfgInstance.Wss.Path, websocket.Handler(w.WebSockHandle))
	http.HandleFunc("/", w.Index)

	utils.Logger.Println("Wss Server going to Start, Server Serial:", w.Serial, " port:", utils.CfgInstance.Wss.Port)

	if utils.CfgInstance.Wss.Private == "" || utils.CfgInstance.Wss.Public == "" {
		utils.Logger.Println("warning: Https未配置，将采用http服务。若需要https请配置好配置文件后重启")
		if err := http.ListenAndServe(":"+utils.CfgInstance.Wss.Port, nil); err != nil {
			utils.Logger.Fatal(err)
		}
	} else {
		if err := http.ListenAndServeTLS(":"+utils.CfgInstance.Wss.Port, utils.CfgInstance.Wss.Private, utils.CfgInstance.Wss.Public, nil); err != nil {
			utils.Logger.Fatal(err)
		}
	}

}
