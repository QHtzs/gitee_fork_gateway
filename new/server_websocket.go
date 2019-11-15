package main

/*
web socket
*/

import (
	"log"
	"net/http"
	"time"

	"golang.org/x/net/websocket"
)

type WebSocketServerEntity struct {
	port        string           //端口
	ToBroadCast []ServerImpl     //广播对象
	Serial      string           //服务号
	ParseInf    PackageParseImpl //消息解析
	Pool        *MemPool         //pool
	MemQueue    chan DataWrapper //消息
	Map         NetConMap        //x
	TimeOutSec  int64            //超时时间

}

func (w *WebSocketServerEntity) Init(port, serial string, can_dup bool, cap_ int, timeoutsec int64, pool *MemPool, pv PackageParseImpl) {
	w.port = port
	w.Serial = serial
	w.ParseInf = pv
	w.ToBroadCast = make([]ServerImpl, 0, cap_)
	w.MemQueue = make(chan DataWrapper, 100)
	w.Pool = pool
	w.TimeOutSec = timeoutsec
	w.Map.SetAllowDup(can_dup)
}

func (w *WebSocketServerEntity) AddToDistributeEntity(v ServerImpl) {
	w.ToBroadCast = append(w.ToBroadCast, v)
}

func (w *WebSocketServerEntity) SerialIsActivity(serial string) bool {
	ok := w.Map.IsKeyExist(serial)
	return ok
}

func (w *WebSocketServerEntity) GetSerial() string {
	return w.Serial
}

func (w *WebSocketServerEntity) AddDataForWrite(data DataWrapper) {
	w.MemQueue <- data
}

func (w *WebSocketServerEntity) BroadCastData(data DataWrapper) {
	for _, obj := range w.ToBroadCast {
		if obj != nil {
			obj.AddDataForWrite(data)
		}
	}
}

func (w *WebSocketServerEntity) SerialActivityMap(serial string) map[string]bool {
	ret := make(map[string]bool, 1+len(w.ToBroadCast))
	ret[w.GetSerial()] = w.SerialIsActivity(serial)
	for _, v := range w.ToBroadCast {
		ret[v.GetSerial()] = v.SerialIsActivity(serial)
	}
	return ret
}

func (w *WebSocketServerEntity) wread(con *websocket.Conn, entity *MemEntity) (int, error) {
	var tmp []byte = make([]byte, 0, 0)
	bytes, _ := entity.Bytes()
	err := websocket.Message.Receive(con, &tmp)
	if err != nil {
		return -1, err
	}

	length := len(tmp)
	for i := 0; i < length; i++ {
		bytes[i] = tmp[i]
	}
	bytes[length] = 0
	return length, nil
}

func (w *WebSocketServerEntity) wwrite(con *websocket.Conn, entity *MemEntity, size int) error {
	bytes, _ := entity.Bytes()
	err := websocket.Message.Send(con, string(bytes[0:size])) //if type of params2 is string html5 get string obj, otherwise Blob
	return err
}

func (w *WebSocketServerEntity) createWriteConroutine() {
	for {
		data := <-w.MemQueue
		serial := data.TargetConSerial
		self_id := data.SelfId
		if self_id == "" {
			w.Map.Range(serial, func(mkey interface{}, mvalue interface{}) bool {
				con, ok := mvalue.(*websocket.Conn)
				if ok && time.Now().Unix()-data.CreateUnixSec < w.TimeOutSec {
					w.wwrite(con, data.DataStore, data.DataLength)
				}
				return true
			})
		} else {
			if v, ok := w.Map.Load(serial, self_id); ok {
				con, ok := v.(*websocket.Conn)
				if ok && time.Now().Unix()-data.CreateUnixSec < w.TimeOutSec {
					w.wwrite(con, data.DataStore, data.DataLength)
				}
			}
		}
		data.DataStore.ReleaseOnece()
	}
}

func (w *WebSocketServerEntity) WebSockHandle(ws *websocket.Conn) {
	entity := w.Pool.GetEntity(1, 1024)
	defer entity.FullRelease()
	defer ws.Close()

	bytes, _ := entity.Bytes()
	length, err := w.wread(ws, entity)

	if length < 5 {
		if err == nil {
			websocket.Message.Send(ws, "serial invalid")
		} else {
			websocket.Message.Send(ws, err.Error())
		}
		return
	}

	if !(bytes[0] == 'T' && bytes[1] == 'T' && bytes[2] == 'G') {
		websocket.Message.Send(ws, "error: serial invalid")
		return
	}

	serial := string(bytes[0:length])
	bytes[0] = 'W'
	bytes[1] = 'E'
	bytes[2] = 'B'
	bytes[3] = '_'
	bytes[4] = 'A'
	bytes[5] = 'C'
	bytes[6] = 'C'
	bytes[7] = 'E'
	bytes[8] = 'P'
	bytes[9] = 'T'
	w.wwrite(ws, entity, 10)

	req_addr := ws.Request().RemoteAddr

	defer w.Map.Delete(serial, req_addr)
	inter, ok := w.Map.LoadOrStore(serial, req_addr, ws)
	if ok { //与tcp策略不同
		if mws, ok := inter.(*websocket.Conn); ok {
			mws.Close()
		}
		w.Map.Store(serial, req_addr, ws)
	}
	log.Println(w.Serial, serial, "connect")

	for {
		length, _ = w.wread(ws, entity)

		if length == -1 {
			log.Println(w.Serial, serial, "dis connect")
			break
		}

		toself := w.Pool.GetEntity(1, 1024)
		tocast := w.Pool.GetEntity(len(w.ToBroadCast), 1024)

		s_size, c_size, serial0, _ := w.ParseInf.Parser(w.Serial, serial, entity, toself, tocast, &length, nil)

		if s_size > 0 {
			s_write := DataWrapper{
				DataStore:       toself,
				UdpAddr:         nil,
				DataLength:      s_size,
				TargetConSerial: serial,
				SelfId:          req_addr,
				CreateUnixSec:   time.Now().Unix(),
			}
			w.AddDataForWrite(s_write)
		} else {
			toself.FullRelease()
		}

		if c_size > 0 {
			c_write := DataWrapper{
				DataStore:       tocast,
				UdpAddr:         nil,
				DataLength:      c_size,
				TargetConSerial: serial0,
				SelfId:          "",
				CreateUnixSec:   time.Now().Unix(),
			}

			w.BroadCastData(c_write)
		} else {
			tocast.FullRelease()
		}
	}
}

func (w *WebSocketServerEntity) Index(hw http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}
	hw.Write(IndexPageBuff)
}

func (w *WebSocketServerEntity) StartListen() {
	log.Println(w.Serial, "address :", w.port)
	http.Handle("/websocket", websocket.Handler(w.WebSockHandle))
	http.HandleFunc("/", w.Index)
	go w.createWriteConroutine()

	if ConfigInstance.Https.Crt == "" || ConfigInstance.Https.Key == "" {
		log.Println("warning: Https未配置，将采用http服务。若需要https请配置好配置文件后重启")
		if err := http.ListenAndServe(":"+w.port, nil); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := http.ListenAndServeTLS(":"+w.port, ConfigInstance.Https.Crt, ConfigInstance.Https.Key, nil); err != nil {
			log.Fatal(err)
		}
	}
}
