package tcpservers

import (
	"cloudserver/utils"
	"container/list"
	"fmt"
	"net"
	"sync"
	"time"
)

//连接认证方法, 返回值参数1:认证失败或者成功， 参数2：认证成功后有效，代表连接id，为连tcpsocket接序列号
type AckSerialFunc func(con net.Conn, buf, buf1 []byte) (bool, string)

//确认连接被接受
type AckConFunc func(con net.Conn, serial string) error

//连接信息广播
type SerialTriggerBroadcast interface {
	DoConDistribute(serial string, v ServerImpl, hd PoolHandle, vs ...ServerImpl)
	DoDisDistribute(serial string, v ServerImpl, hd PoolHandle, vs ...ServerImpl)
}

//连接断开时callback
type DisConnectCallBack func(serial string)

//业务解析处理函数接口
type RecvParseInte interface {
	ParseRecvData(serial string, src, dst []byte, src_len *int) int //解析接收的字节并把需要发送到其它服务器的任务数据写入dst
}

//加密解密接口, tmp为借用的内存，加密解密后内容仍然在src中
type De_En_Crypt interface {
	EncryPt(src, tmp []byte, src_len, tmp_len int) (int, bool) //发送数据加密,块加密时，请注意池piece_size不宜过小,否则会溢出
	DeCrypt(src, tmp []byte, src_len, tmp_len int) (int, bool) //发送数据解密
}

//心跳逻辑接口
type BeatInter interface {
	GenAckBeatBytes() []byte //生成心跳确认包
	BeatFreqSec() int64      //tcp心跳时间(秒)
}

//不对redis处理做解耦
type ServerImpl interface {
	Init(_cap int)                                                                                                        //初始化
	SettingSerial(serial string)                                                                                          //设置serial
	SettingBroadCastType(force bool)                                                                                      //分发类型，是否强制检验数据被客户端正确接收
	SettingDecryptObject(v De_En_Crypt)                                                                                   //添加加密解密
	SettingBeatObject(v BeatInter)                                                                                        //添加心跳生成及检测接口
	SettingWriteType(pool_size int)                                                                                       //是否采用写入池
	SettingRecvParseObject(v RecvParseInte)                                                                               //添加业务解析接口
	SettingConTrigger(v SerialTriggerBroadcast)                                                                           //连接断开触发
	AddToDistribute(v ServerImpl)                                                                                         //对象关联, v为接收句柄对象,以及数据交互是否加密
	StartServer(addr string, timeout int64, ptr *PoolHandle, f AckSerialFunc, f1 AckConFunc, callback DisConnectCallBack) //开启服务
	SerialIsActivity(serial string) bool                                                                                  //判断socket是否连接
	AddDataToWrite(p PoolHandle, data_size int, serial string)                                                            //添加待处理(写入等)的数据句柄到单个对象
	GetSerial() string                                                                                                    //
	DistributeDataToWrite(p PoolHandle, data_size int, serial string)                                                     //
}

//待发送数据
type DataInfoHandle struct {
	Handle          PoolHandle //池
	Length          int        //数据长度
	ConnFd          string     //需要发送到哪个socket
	RecvTimeUnixSec int64      //接收时间，用于校验是否超时
}

func (d *DataInfoHandle) IsTimeOut(timeout_sec int64) bool {
	now := time.Now().Unix()
	return now-d.RecvTimeUnixSec > timeout_sec
}

//不采用chan + select采用轮询阻塞的方式
type MFiFoQueue struct {
	mutex     sync.Mutex
	List      *list.List
	MultiList map[string]*list.List
}

//初始化队列
func (m *MFiFoQueue) New() {
	m.List = list.New()
	m.MultiList = make(map[string]*list.List, 1)
}

//添加
func (m *MFiFoQueue) Push(d DataInfoHandle) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.List.PushBack(d)
}

//取出
func (m *MFiFoQueue) Get() DataInfoHandle {
	var res DataInfoHandle
	for {
		m.mutex.Lock()
		size := m.List.Len()
		if size > 0 {
			v := m.List.Remove(m.List.Front())
			res = v.(DataInfoHandle)
			m.mutex.Unlock()
		} else {
			m.mutex.Unlock()
		}
		time.Sleep(200 * time.Millisecond)
	}
	return res
}

//添加1
func (m *MFiFoQueue) Put(id string, d DataInfoHandle) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if v, ok := m.MultiList[id]; ok {
		v.PushBack(d)
	} else {
		l := list.New()
		l.PushBack(d)
		m.MultiList[id] = l
	}
}

//取出1
func (m *MFiFoQueue) Pop(id string) interface{} {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if v, ok := m.MultiList[id]; ok {
		if v.Len() <= 0 {
			return nil
		} else {
			return v.Remove(v.Front())
		}
	} else {
		return nil
	}
}

//读取队列(池)中的函数及变量，并执行(把连接处理池化)
type NewConnectHandle struct {
	T   *TcpServerEntity
	F0  AckSerialFunc
	F1  AckConFunc
	Cbk DisConnectCallBack
	Con net.Conn
}

//执行
func (n *NewConnectHandle) Exec(buf, buf1 []byte) {
	ok, serial := n.F0(n.Con, buf, buf1)
	if !ok {
		fmt.Println(n.T.Serial, " 连接认证失败")
		n.Con.Close()
		return
	}

	if serial[0] == 'M' && len(serial) > 8 {
		serial = serial[8:] //MONITOR_
		n.T.addMonitor(serial, n.Con)

	} else {
		//若连接存在，则强制断开连接,等待下一次连接再接收
		if n.T.forceDisconnect(serial) {
			fmt.Println(n.T.Serial, "已经断开，socket serial=", serial, "请重新连接")
			return
		}
		err := n.F1(n.Con, serial)
		if err != nil {
			return
		}
		fmt.Println(n.T.Serial, " 连接成功，socket serial=", serial)
		n.T.addNewConnInfo(serial, n.Con)

		if n.T.Trigger != nil {
			hd := n.T.PoolPtr.Brrow()
			n.T.Trigger.DoConDistribute(serial, n.T, hd, n.T.ObjSlice...)
		}

		if n.T.WritePoolSize < 1 {
			go n.T.createWriteRoutin(n.Con, serial)
		}
		go n.T.createReadRoutin(n.Con, serial, n.Cbk)
	}
}

//服务器实例, 默认允许被挤下线
type TcpServerEntity struct {
	Listener      net.Listener           //服务监听器
	Buff          MFiFoQueue             //缓冲队列
	TcpConn       sync.Map               //长连接soketfd, socketcon
	TimeOutSec    int64                  //设置超时时间秒
	Mps           map[string]*int64      //上一次发送心跳包时间soketfd, time
	mutex         sync.RWMutex           //读写锁
	ObjSlice      []ServerImpl           //关联的包
	Crypt         De_En_Crypt            //加密接口
	Beats         BeatInter              //tcp辅助心跳接口
	Parser        RecvParseInte          //业务解析器
	Trigger       SerialTriggerBroadcast //
	ForceAck      bool                   //broadcast类型,是否需要反馈。需要反馈则数据发送到成功为止，不需要反馈则发送完即可
	WritePoolSize int                    //写入线程池大小， <= 0则没有采用池模式
	Started       bool                   //是否已经开始监听，若开始监听则不允许修改参数
	Serial        string                 //该服务id
	PoolPtr       *PoolHandle            //poolhand
	MonitorCons   sync.Map               //监控连接,支持多个连接
	AcceptChan    chan NewConnectHandle
}

//初始化
func (t *TcpServerEntity) Init(_cap int) {
	t.Started = false
	t.Serial = ""
	t.ObjSlice = make([]ServerImpl, 0, _cap)
	t.Mps = make(map[string]*int64, 1000)
	t.Crypt = nil
	t.Beats = nil
	t.ForceAck = false
	t.WritePoolSize = 0
	t.Buff = MFiFoQueue{}
	t.Buff.New()
	t.PoolPtr = nil
	t.Parser = nil
	t.Trigger = nil
	t.AcceptChan = make(chan NewConnectHandle, 100)

}

//设置服务标志(序列号)
func (t *TcpServerEntity) SettingSerial(serial string) {
	t.Serial = serial
}

//设置广播是否强制被确认
func (t *TcpServerEntity) SettingBroadCastType(force bool) {
	if !t.Started {
		t.ForceAck = force
	}
}

//添加加密解密接口
func (t *TcpServerEntity) SettingDecryptObject(v De_En_Crypt) {
	if !t.Started {
		t.Crypt = v
	}
}

//添加心跳生成及检测接口
func (t *TcpServerEntity) SettingBeatObject(v BeatInter) {
	if !t.Started {
		t.Beats = v
	}
}

//创建写入池, 多个长连接共享写入goroutin池. pool_size小于等于0则不采用该模式
func (t *TcpServerEntity) SettingWriteType(pool_size int) {
	if !t.Started {
		t.WritePoolSize = pool_size
	}
}

//业务解析器
func (t *TcpServerEntity) SettingRecvParseObject(v RecvParseInte) {
	t.Parser = v
}

//连接断开时触发
func (t *TcpServerEntity) SettingConTrigger(v SerialTriggerBroadcast) {
	t.Trigger = v
}

//对象关联, v为接收句柄对象,以及数据交互是否加密心跳处理等
func (t *TcpServerEntity) AddToDistribute(v ServerImpl) {
	if !t.Started {
		t.ObjSlice = append(t.ObjSlice, v)
	}
}

//数据分发
func (t *TcpServerEntity) DistributeDataToWrite(p PoolHandle, data_size int, serial string) {
	for _, obj := range t.ObjSlice {
		obj.AddDataToWrite(p, data_size, serial)
	}
}

//判断serial为x的socket连接是否存在
func (t *TcpServerEntity) SerialIsActivity(serial string) bool {
	_, ok := t.TcpConn.Load(serial)
	return ok
}

//开启服务,addr string:端口, timeout int64:数据有效时间, f AckSerialFunc:认证逻辑处理函数
func (t *TcpServerEntity) StartServer(addr string, timeout int64, ptr *PoolHandle, f AckSerialFunc, f1 AckConFunc, callback DisConnectCallBack) {
	t.PoolPtr = ptr
	t.Started = true
	address := ":" + addr
	if t.Serial == "" {
		t.Serial = "server serial:" + address
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		panic("tcp server failed to start")
	}
	t.Listener = listener
	t.TimeOutSec = timeout

	if t.WritePoolSize > 0 {
		t.createWriteAllShare()
	}
	go t.createBeatRoutin()
	for i := 0; i < utils.ConfigInstance.Other.ConfirmSize; i++ {
		go t.consumerAccept() //serial确认任务池化,减少不必要的阻塞,一定程度缓解突发性多个连接同时访问问题
	}
	for {
		con, err := t.Listener.Accept()
		if err != nil {
			fmt.Println("创建连接失败")
			continue
		}
		handle := NewConnectHandle{
			T:   t,
			F0:  f,
			F1:  f1,
			Cbk: callback,
			Con: con,
		}
		t.AcceptChan <- handle
	}

}

//添加监控客户端
func (t *TcpServerEntity) addMonitor(serial string, con net.Conn) {
	v, ok := t.MonitorCons.LoadOrStore(serial, con)
	if ok {
		con.Close()
		con, ok = v.(net.Conn)
		if ok {
			con.Close()
		}
	}
}

//移除监控客户端
func (t *TcpServerEntity) removeMonitor(serial string) {
	v, ok := t.MonitorCons.Load(serial)
	if ok {
		con, ok := v.(net.Conn)
		if ok {
			con.Close()
		}
	}
	t.MonitorCons.Delete(serial)
}

//添加新连接
func (t *TcpServerEntity) addNewConnInfo(serial string, con net.Conn) {
	value := new(int64)
	*value = time.Now().Unix()

	t.mutex.Lock() //上读锁
	t.Mps[serial] = value
	t.mutex.Unlock()

	t.TcpConn.Store(serial, con)
}

//移除连接
func (t *TcpServerEntity) removeConnInfo(serial string) {
	t.mutex.Lock() //上读锁
	if _, ok := t.Mps[serial]; ok {
		delete(t.Mps, serial)
	}
	t.mutex.Unlock()
	t.TcpConn.Delete(serial)
}

//强制断开连接
func (t *TcpServerEntity) forceDisconnect(serial string) bool {
	v, ok := t.TcpConn.Load(serial)
	status := false
	if ok {
		v.(net.Conn).Close()
		status = true
	}
	return status
}

//获取serial状态
func (t *TcpServerEntity) GetSerial() string {
	return t.Serial
}

//添加待处理的数据句柄到单个对象,处理后记得释放
func (t *TcpServerEntity) AddDataToWrite(p PoolHandle, data_size int, serial string) {
	data := DataInfoHandle{
		Handle:          p,
		Length:          data_size,
		ConnFd:          serial,
		RecvTimeUnixSec: time.Now().Unix(),
	}

	if t.WritePoolSize == 0 {
		t.Buff.Put(serial, data)
	} else {
		t.Buff.Push(data)
	}

}

//等待协程
func (t *TcpServerEntity) consumerAccept() {
	buf1 := make([]byte, 128)
	buf := make([]byte, 128)
	var handle NewConnectHandle
	for {
		select {
		case handle = <-t.AcceptChan:
			handle.Exec(buf, buf1)
		}
	}
}

//心跳数据发送, interface为nil不进行心跳
func (t *TcpServerEntity) createBeatRoutin() {
	if t.Beats != nil {
		alive := t.Beats.GenAckBeatBytes()
		freq := t.Beats.BeatFreqSec()
		var buf []byte
		if t.Crypt != nil {
			size := len(alive)
			buf = make([]byte, 8*size, 8*size)
			buf1 := make([]byte, 8*size, 8*size)
			for i := 0; i < size; i++ {
				buf[i] = alive[i]
			}
			size, ok := t.Crypt.EncryPt(buf, buf1, size, 8*size)
			if !ok {
				panic("心跳包加密失败")
			}
			alive = buf[0:size]
			buf = nil
			buf1 = nil
		}
		for {
			now := time.Now().Unix() //数据发送较快，心跳发送时差要求不高，故一轮过后now时间基本相等，不重复调用time.Now().Unix()
			t.mutex.RLock()
			for k, v := range t.Mps {
				if now-*v > freq {
					*v = now
					obj, ok := t.TcpConn.Load(k)
					if ok {
						con, ok := obj.(net.Conn)
						if ok {
							con.Write(alive)
						}
					}
				}
			}
			t.mutex.RUnlock()
			time.Sleep(100 * time.Millisecond)
		}
	}
}

//写入池式的
func (t *TcpServerEntity) createWriteAllShare() {
	for i := 0; i < t.WritePoolSize; i++ {
		go func(tt *TcpServerEntity) {
			for {
				data := tt.Buff.Get()
				if data.IsTimeOut(tt.TimeOutSec) {
					data.Handle.ReleaseOnce()
					continue
				}
				bytes := data.Handle.GetBytes()
				length := data.Length
				serial := data.ConnFd
				con_v, ok := tt.TcpConn.Load(serial)
				if tt.ForceAck {
					if ok {
						if tt.Crypt != nil {
							hd2 := data.Handle.BrrowAndAcq(1)
							tmp_bytes := hd2.GetBytes()
							length, ok = t.Crypt.EncryPt(bytes, tmp_bytes, length, len(tmp_bytes))
							hd2.ReleaseOnce()
							if !ok {
								panic("加密错误，请确认加密函数是否正确设置，如tmp超限之类")
							}
						}
						con := con_v.(net.Conn)

						fmt.Println(t.Serial, "write data byte:", string(bytes[0:length]))
						_, err := con.Write(bytes[0:length])

						monit_con, ok := t.MonitorCons.Load(serial)
						if ok {
							m_con, _ := monit_con.(net.Conn)
							_, werr := m_con.Write(bytes[0:length])
							if werr != nil {
								t.removeMonitor(serial)
							}
						}

						if err == nil {
							data.Handle.ReleaseOnce()
							continue
						}
					} else {
						tt.Buff.Push(data) //不释放
					}

				} else {
					if ok {
						if tt.Crypt != nil {
							hd2 := data.Handle.BrrowAndAcq(1)
							tmp_bytes := hd2.GetBytes()
							length, ok = t.Crypt.EncryPt(bytes, tmp_bytes, length, len(tmp_bytes))
							hd2.ReleaseOnce()
							if !ok {
								panic("加密错误，请确认加密函数是否正确设置，如tmp超限之类")
							}
						}
						con := con_v.(net.Conn)

						fmt.Println(t.Serial, "write data byte:", string(bytes[0:length]))
						con.Write(bytes[0:length])

						monit_con, ok := t.MonitorCons.Load(serial)
						if ok {
							m_con, _ := monit_con.(net.Conn)
							_, werr := m_con.Write(bytes[0:length])
							if werr != nil {
								t.removeMonitor(serial)
							}
						}
					}
					data.Handle.ReleaseOnce()
				}

			}
		}(t)
	}
}

//写入池式（非goroutin池）
func (t *TcpServerEntity) createWriteRoutin(con net.Conn, serial string) {
	for {
		itf := t.Buff.Pop(serial)
		if itf == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		data, ok := itf.(DataInfoHandle)

		if !ok {
			fmt.Println("warning:队列取数有点问题")
			continue
		}
		if data.IsTimeOut(t.TimeOutSec) {
			data.Handle.ReleaseOnce()
			continue
		}
		bytes := data.Handle.GetBytes()
		length := data.Length
		serial := data.ConnFd

		_, ok = t.TcpConn.Load(serial)
		need_exit := !ok

		if t.ForceAck {
			if ok {
				if t.Crypt != nil {
					hd2 := data.Handle.BrrowAndAcq(1)
					tmp_bytes := hd2.GetBytes()
					length, ok = t.Crypt.EncryPt(bytes, tmp_bytes, length, len(tmp_bytes))
					hd2.ReleaseOnce()
					if !ok {
						panic("加密错误，请确认加密函数是否正确设置，如tmp超限之类")
					}
				}

				fmt.Println(t.Serial, "write data byte:", string(bytes[0:length]))
				_, err := con.Write(bytes[0:length])

				monit_con, ok := t.MonitorCons.Load(serial)
				if ok {
					m_con, _ := monit_con.(net.Conn)
					_, werr := m_con.Write(bytes[0:length])
					if werr != nil {
						t.removeMonitor(serial)
					}
				}

				if err == nil {
					data.Handle.ReleaseOnce()
					continue
				}
			} else {
				t.Buff.Put(serial, data) //不释放
			}

		} else {
			if ok {
				if t.Crypt != nil {
					hd2 := data.Handle.BrrowAndAcq(1)
					tmp_bytes := hd2.GetBytes()
					length, ok = t.Crypt.EncryPt(bytes, tmp_bytes, length, len(tmp_bytes))
					hd2.ReleaseOnce()
					if !ok {
						panic("加密错误，请确认加密函数是否正确设置，如tmp超限之类")
					}
				}

				fmt.Println(t.Serial, "write data byte:", string(bytes[0:length]))
				con.Write(bytes[0:length])

				monit_con, ok := t.MonitorCons.Load(serial)
				if ok {
					m_con, _ := monit_con.(net.Conn)
					_, werr := m_con.Write(bytes[0:length])
					if werr != nil {
						t.removeMonitor(serial)
					}
				}
			}
			data.Handle.ReleaseOnce()
		}

		//routin 需要退出机制,连接断开则退出
		if need_exit {
			break
		}
	}
}

//创建读任务handle, 读取任务是阻塞的，不做池处理。 抛错逻辑由此处处理
func (t *TcpServerEntity) createReadRoutin(con net.Conn, serial string, callback DisConnectCallBack) {
	hd := t.PoolPtr.BrrowAndAcq(1)
	src_len := 0
	var a, b, c byte = 0, 0, 0
	mod := 0
	for {
		if src_len == 1024 {
			fmt.Println("warning:数据丢包严重，或者选取buf长度不够")
			src_len = 0
		}
		length, err := con.Read(hd.GetBytes()[src_len+mod:])
		if err != nil {
			con.Close()
			fmt.Println("close con id=", serial)
			t.removeConnInfo(serial)
			if callback != nil {
				callback(serial)
			}
			if t.Trigger != nil {
				t.Trigger.DoDisDistribute(serial, t, hd, t.ObjSlice...)
			}
			break
		}
		hd1 := hd.BrrowAndAcq(len(t.ObjSlice))
		if t.Crypt != nil {
			m_bytes := hd.GetBytes()
			if mod == 0 {
				//最早跳出
			} else if mod == 3 {
				m_bytes[src_len] = a
				m_bytes[src_len+1] = b
				m_bytes[src_len+2] = c
			} else if mod == 2 {
				m_bytes[src_len] = a
				m_bytes[src_len+1] = b
			} else if mod == 1 {
				m_bytes[src_len] = a
			}
			length += mod
			if length%4 > 0 {
				mod = length % 4
				if mod == 0 {
					//最早跳出
				} else if mod == 1 {
					a = m_bytes[src_len+length-1]
				} else if mod == 2 {
					b = m_bytes[src_len+length-1]
					a = m_bytes[src_len+length-2]
				} else if mod == 3 {
					c = m_bytes[src_len+length-1]
					b = m_bytes[src_len+length-2]
					a = m_bytes[src_len+length-3]
				}
			}
			length, _ = t.Crypt.DeCrypt(m_bytes[src_len:], hd1.GetBytes(), length-mod, length)

		}
		length += src_len

		// fmt.Println("buf remain:", hd.GetBytes())

		l := t.Parser.ParseRecvData(serial, hd.GetBytes(), hd1.GetBytes(), &length)
		src_len = length

		for _, obj := range t.ObjSlice {
			if l > 0 {

				obj.AddDataToWrite(hd1, l, serial)

				monit_con, ok := t.MonitorCons.Load(serial)
				if ok {
					m_con, _ := monit_con.(net.Conn)
					_, werr := m_con.Write(hd1.GetBytes()[0:l])
					if werr != nil {
						t.removeMonitor(serial)
					}
				}

			} else {
				hd1.ReleaseOnce()
			}
		}
	}

	hd.ReleaseOnce()
}
