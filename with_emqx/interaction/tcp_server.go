package interaction

import (
	"cloudserver/with_emqx/utils"
	"net"
)

//ConImp接口实现,针对tcp
type TCon struct {
	serial string
	Isok   bool
	net.Conn
}

func (t *TCon) SetSerial(serial string) {
	t.serial = serial
}

func (t *TCon) Serial() string {
	return t.serial
}

func (t *TCon) Ok() bool {
	return t.Isok
}

func (t *TCon) Close() error {
	t.Isok = false
	return t.Conn.Close()
}

func (t *TCon) PeerAddress() string {
	return t.Conn.RemoteAddr().String()
}

//ServerImp接口实现, tcp服务
type TcpServer struct {
	Cons           utils.NetConMap                  //已经连接的上位机fd
	Queue          chan ConImp                      //待处理连接数
	Serial         string                           //服务名
	Listen         net.Listener                     //监听器
	ReadAndPublish func(serial string, data []byte) //消息推送
	NewConCallBack func(t ServerImp, con ConImp)    //新建连接
	DisConCallBack func(t ServerImp, con ConImp)    //连接断开
	AuthFunc       func(con ConImp) bool            //授权
	Port           string                           //端口
}

//接口
func (s *TcpServer) ClientSerial() string {
	return s.Serial
}

//接口
func (s *TcpServer) ClientIsOnline(serial string) bool {
	return s.Cons.IsKeyExist(serial)
}

//接口
func (s *TcpServer) Publish(serial string, data []byte) {
	cons := s.LoadCons(serial)
	for _, c := range cons {
		c.Write(data)
	}
}

//断开连接，并触发DisConCallBack
func (s *TcpServer) CloseCon(con ConImp) {
	con.Close()
	s.Cons.Delete(con.Serial(), con.PeerAddress())
	if s.DisConCallBack != nil {
		s.DisConCallBack(s, con)
	}
}

//建立连接， 并触发NewConCallBack
func (s *TcpServer) AddCon(con ConImp) bool {
	_, ok := s.Cons.LoadOrStore(con.Serial(), con.PeerAddress(), con)
	if !ok && s.NewConCallBack != nil {
		s.NewConCallBack(s, con)
	}
	return !ok
}

//获取同 主机序列号下的所有上位机控制连接 [主机:控制 == 1:n]
func (s *TcpServer) LoadCons(serial string) []ConImp {
	cons := make([]ConImp, 0, 1)
	s.Cons.Range(serial, func(k, v interface{}) bool {
		cn := v.(ConImp)
		cons = append(cons, cn)
		return true
	})
	return cons
}

//初始化参数
func (s *TcpServer) Init(allow_dup bool, ncbk, dcbk func(t ServerImp, con ConImp), pub func(serial string, data []byte), authandsetserial func(con ConImp) bool) {
	s.Cons.SetAllowDup(true)
	s.NewConCallBack = ncbk
	s.DisConCallBack = dcbk
	s.ReadAndPublish = pub
	s.AuthFunc = authandsetserial
	s.Queue = make(chan ConImp, 300)
}

//读取上位机推送控制消息，并推送到mqtt
func (s *TcpServer) ReadConAndPublish(con ConImp) {
	buf := make([]byte, 1024, 1024)
	var sz int
	var err error
	for con.Ok() {
		sz, err = con.Read(buf)
		if err != nil {
			s.CloseCon(con)
			break
		}
		if err == nil && sz > 0 && s.ReadAndPublish != nil {
			s.ReadAndPublish(con.Serial(), buf[0:sz])
		}
	}
}

//创建授权的routine
func (s *TcpServer) StartAuthRoutine(routine_size int) {
	for i := 0; i < routine_size; i++ {
		go func(v *TcpServer) {
			for {
				con := <-v.Queue
				if v.AuthFunc != nil && v.AuthFunc(con) {
					ok := v.AddCon(con)
					if !ok {
						con.Write([]byte("con exist"))
						con.Close()
					} else {
						go s.ReadConAndPublish(con)
					}
				} else {
					con.Close()
				}
			}
		}(s)
	}
}

//开始监听
func (s *TcpServer) StartListen() {
	utils.Logger.Println("Tcp Server Started, Server Serial:", s.Serial, " port:", s.Port)
	l, err := net.Listen("tcp", ":"+s.Port)
	if err != nil {
		utils.Logger.Fatal(err)
	}
	s.Listen = l
	for {
		c, err := s.Listen.Accept()
		if err != nil {
			utils.Logger.Println(err)
		}
		con := &TCon{Conn: c, Isok: true}
		s.Queue <- con
	}
}
