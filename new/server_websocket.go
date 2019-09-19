package main

/*
web socket
*/

import (
	"html/template"
	"log"
	"net/http"
	"sync"
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
	Map         sync.Map         //x
	TimeOutSec  int64            //超时时间

}

func (w *WebSocketServerEntity) Init(port, serial string, cap_ int, timeoutsec int64, pool *MemPool, pv PackageParseImpl) {
	w.port = port
	w.Serial = serial
	w.ParseInf = pv
	w.ToBroadCast = make([]ServerImpl, 0, cap_)
	w.MemQueue = make(chan DataWrapper, 100)
	w.Pool = pool
	w.TimeOutSec = timeoutsec
}

func (w *WebSocketServerEntity) AddToDistributeEntity(v ServerImpl) {
	w.ToBroadCast = append(w.ToBroadCast, v)
}

func (w *WebSocketServerEntity) SerialIsActivity(serial string) bool {
	_, ok := w.Map.Load(serial)
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
		if v, ok := w.Map.Load(serial); ok {
			con, ok := v.(*websocket.Conn)
			if ok && time.Now().Unix()-data.CreateUnixSec < w.TimeOutSec {
				w.wwrite(con, data.DataStore, data.DataLength)
			}
		}
		data.DataStore.ReleaseOnece()
	}
}

func (w *WebSocketServerEntity) WebSockHandle(ws *websocket.Conn) {
	entity := w.Pool.GetEntity(1, 1024)
	defer entity.FullRelease()

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

	w.Map.Store(serial, ws)
	defer w.Map.Delete(serial)

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
	t, _ := template.ParseFiles("index.html")
	t.Execute(hw, nil)
}

func (w *WebSocketServerEntity) StartListen() {
	log.Println(w.Serial, "address :", w.port)
	http.Handle("/websocket", websocket.Handler(w.WebSockHandle))
	http.HandleFunc("/", w.Index)
	go w.createWriteConroutine()
	if err := http.ListenAndServe(":"+w.port, nil); err != nil {
		log.Fatal(err)
	}
}

type WebSocketParse struct {
}

func (w *WebSocketParse) Parser(server_serial, cur_con_serial string, src, toself, tocast *MemEntity, src_len *int, v CryptImpl) (s_size int, c_size int, serial string, beat bool) {
	bytes, _ := src.Bytes()
	sbytes, _ := toself.Bytes()
	cbytes, _ := tocast.Bytes()
	for i := 0; i < *src_len; i++ {
		cbytes[i] = bytes[i]
		sbytes[i] = bytes[i]
	}
	return *src_len, *src_len, cur_con_serial, false
}
