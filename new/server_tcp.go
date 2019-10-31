package main

import (
	"log"
	"net"
	"sync"
	"time"
)

type WrapConn struct {
	closed bool
	net.Conn
}

func (w *WrapConn) Close() {
	w.closed = true
	w.Conn.Close()
}

func (w *WrapConn) IsClosed() bool {
	return w.closed
}

type TcpServerEntity struct {
	Listener           net.Listener          //服务监听器
	BeatInf            BeatPackageImpl       //心跳处理接口
	CryptInf           CryptImpl             //加密解密接口
	ObserverInf        ConChangeObserverImpl //观察接口
	AckInf             AckImpl               //连接确认接口
	ParserInf          PackageParseImpl      //解析接口
	ToBroadCast        []ServerImpl          //广播对象
	LastBeatSend       map[string]*int64     //上一次发送心跳包时间(soketfd:time)
	MemQueue           FifoQueue             //数据存放队列
	ChanQueue          chan net.Conn         //chan消息队列
	ChanRoutineStatus  [100]bool             //灵活拓展routine用于应对，瞬时多个连接, 不加锁粗略使用
	Pool               *MemPool              //内存池
	MonitorCons        sync.Map              //监控某个socket的con
	TcpClientCons      NetConMap             //tcp连接
	Serial             string                //服务序列号
	TimeOutSec         int64                 //超时时间 秒
	WriteConrutionSize int                   //写数据的conroution数，少于0则每个tcp连接取一个conroutin，反之则共享WriteConrutionSize个
	NeedFeedBack       bool                  //是否需要确认消息被接收
	rw_mutex           sync.RWMutex          //读写锁
	tcpport            string                //服务开启地址
}

func (t *TcpServerEntity) Init(port, serial string, can_dup, needfb bool, con_queue_pool_size, writeConrutionSize, broadCastSize int, timeoutsec int64, pool *MemPool, bv BeatPackageImpl, cv CryptImpl, ov ConChangeObserverImpl, av AckImpl, pv PackageParseImpl) {
	t.tcpport = port
	t.Serial = serial
	t.WriteConrutionSize = writeConrutionSize
	t.ChanQueue = make(chan net.Conn, con_queue_pool_size) //该参数比较重要，实际应用中需要调
	t.TimeOutSec = timeoutsec
	t.BeatInf = bv
	t.CryptInf = cv
	t.ObserverInf = ov
	t.AckInf = av
	t.ParserInf = pv
	t.NeedFeedBack = needfb
	t.Pool = pool
	t.MemQueue = FifoQueue{}
	t.ToBroadCast = make([]ServerImpl, 0, broadCastSize)
	t.LastBeatSend = make(map[string]*int64, 1000)
	t.TcpClientCons.SetAllowDup(can_dup)
}

func (t *TcpServerEntity) AddToDistributeEntity(v ServerImpl) {
	t.ToBroadCast = append(t.ToBroadCast, v)
}

func (t *TcpServerEntity) SerialIsActivity(serial string) bool {
	ok := t.TcpClientCons.IsKeyExist(serial)
	return ok
}

func (t *TcpServerEntity) GetSerial() string {
	return t.Serial
}

func (t *TcpServerEntity) AddDataForWrite(data DataWrapper) {
	if t.WriteConrutionSize > 0 {
		t.MemQueue.SingelPut(data)
	} else {
		t.MemQueue.Put(data.TargetConSerial, data)
	}
}

func (t *TcpServerEntity) BroadCastData(data DataWrapper) {
	for _, obj := range t.ToBroadCast {
		if obj != nil {
			obj.AddDataForWrite(data)
		}
	}
}

func (t *TcpServerEntity) SerialActivityMap(serial string) map[string]bool {
	ret := make(map[string]bool, 1+len(t.ToBroadCast))
	ret[t.GetSerial()] = t.SerialIsActivity(serial)
	for _, v := range t.ToBroadCast {
		ret[v.GetSerial()] = v.SerialIsActivity(serial)
	}
	return ret
}

//添加con
func (t *TcpServerEntity) addMontitor(serial string, con *WrapConn) {
	v, ok := t.MonitorCons.LoadOrStore(serial, con)
	if ok { //两个连接都关闭
		t.MonitorCons.Delete(serial)
		con.Close()
		con, ok = v.(*WrapConn)
		if ok {
			con.Close()
		}
	}
}

func (t *TcpServerEntity) addTcpConn(serial string, con *WrapConn) {
	value := new(int64)
	*value = time.Now().Unix()

	t.rw_mutex.Lock()
	t.LastBeatSend[serial] = value
	t.rw_mutex.Unlock()

	v, ok := t.TcpClientCons.LoadOrStore(serial, con.RemoteAddr().String(), con)
	if ok {
		t.TcpClientCons.Delete(serial, con.RemoteAddr().String())
		con.Close()
		con, ok = v.(*WrapConn)
		if ok {
			con.Close()
		}
	}
}

//移除con
func (t *TcpServerEntity) removeMontitor(serial string) {
	v, ok := t.MonitorCons.Load(serial)
	if ok {
		con, ok := v.(*WrapConn)
		if ok {
			con.Close()
		}
	}
	t.MonitorCons.Delete(serial)
}

func (t *TcpServerEntity) removeTcpConn(serial string, con *WrapConn) {
	t.rw_mutex.Lock()
	if _, ok := t.LastBeatSend[serial]; ok {
		delete(t.LastBeatSend, serial)
	}
	t.rw_mutex.Unlock()
	t.TcpClientCons.Delete(serial, con.RemoteAddr().String())
}

//心跳发送
func (t *TcpServerEntity) createBeatSendHandle() {
	if t.BeatInf != nil {
		beat := t.BeatInf.BeatBytes()
		freq := t.BeatInf.BeatInteval()

		if freq < 1 { //时间间隔小于1，不主动发心跳，只做被动响应
			return
		}

		if t.CryptInf != nil {
			size := len(beat)
			buf0 := make([]byte, 8*size, 8*size)
			buf1 := make([]byte, 8*size, 8*size)
			for i := 0; i < size; i++ {
				buf0[i] = beat[i]
			}
			ok, size, _ := t.CryptInf.EncryPt(buf0, buf1, size, 8*size)
			if !ok {
				panic("心跳包加密失败")
			}
			beat = buf0[0:size]
			buf0 = nil
			buf1 = nil
		}

		for {
			now := time.Now().Unix()
			t.rw_mutex.RLock()
			for k, v := range t.LastBeatSend {
				if now-*v >= freq {
					*v = now
					t.TcpClientCons.Range(k, func(mkey interface{}, mvalue interface{}) bool {
						con, ok := mvalue.(*WrapConn)
						if ok {
							con.Write(beat)
						}
						return true
					})
				}
			}
			t.rw_mutex.RUnlock()
			time.Sleep(time.Duration(freq*800) * time.Millisecond)
		}
	}
}

//心跳响应
func (t *TcpServerEntity) sendBeatAck(con *WrapConn, ack []byte) {
	_, err := con.Write(ack)
	if err != nil {
		con.Close()
	}
}

func (t *TcpServerEntity) createReadroutine(con *WrapConn, serial string) {
	hd := t.Pool.GetEntity(1, 1024)
	defer hd.ReleaseOnece()
	m_byte, _ := hd.Bytes()
	src_len := 0
	tmp_bool := true
	var beat_ack []byte = nil
	if t.BeatInf != nil {
		beat_ack = t.BeatInf.BeatAckBytes()
	}
	for {

		if src_len >= 1024 {
			log.Println("warning:数据丢包严重，或者选取buf长度不够 `buf清空`")
			src_len = 0
		}

		rcv_len, err := con.Read(m_byte[src_len:])
		src_len += rcv_len

		if err != nil {
			log.Println("server[", t.Serial, "] close connect:", serial)
			t.removeTcpConn(serial, con)
			con.Close()
			if t.ObserverInf != nil { //观察存在时
				t.ObserverInf.HDisConnect(serial, t)
				t.ObserverInf.SDisConnect(serial, hd, t, t.ToBroadCast...)
			}
			break
		}

		if rcv_len == 0 && src_len < 64 {
			continue
		}

		tmp_bool = true
		for tmp_bool {
			toself := t.Pool.GetEntity(1, 1024)
			tocast := t.Pool.GetEntity(len(t.ToBroadCast), 1024)

			s_size, c_size, serial0, need_beat := t.ParserInf.Parser(t.Serial, serial, hd, toself, tocast, &src_len, t.CryptInf)

			tmp_bool = c_size > 0 || need_beat

			log.Println(t.Serial, "parser:", s_size, c_size, serial0, need_beat, src_len)

			if need_beat && beat_ack != nil {
				t.sendBeatAck(con, beat_ack)
			}

			if s_size > 0 {
				s_write := DataWrapper{
					DataStore:       toself,
					UdpAddr:         nil,
					DataLength:      s_size,
					TargetConSerial: serial,
					SelfId:          con.RemoteAddr().String(),
					CreateUnixSec:   time.Now().Unix(),
				}
				t.AddDataForWrite(s_write)
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
				t.BroadCastData(c_write)
			} else {
				tocast.FullRelease()
			}
		}
	}
}

//数据写入对应的tcp中
func (t *TcpServerEntity) writeData(data DataWrapper) bool {

	ret := false
	if time.Now().Unix()-data.CreateUnixSec > t.TimeOutSec {
		ret = true
		return ret
	}

	serial := data.TargetConSerial
	src, _ := data.DataStore.Bytes()

	log.Println(t.Serial, "-->", serial, ":", string(src[0:data.DataLength]))

	t.TcpClientCons.Range(serial, func(mkey interface{}, mvalue interface{}) bool {
		con, ok := mvalue.(*WrapConn)
		if ok {
			if con.IsClosed() {
				return true
			}
			if t.CryptInf != nil {

				tmp := data.DataStore.PoolPtr.GetEntity(1, 4*data.DataLength)
				defer tmp.FullRelease()

				dst, dlen := tmp.Bytes()
				status, _, dsize := t.CryptInf.EncryPt(src, dst, data.DataLength, dlen)

				if status {
					if _, err := con.Write(dst[0:dsize]); err == nil {
						ret = true
					} else {
						con.Close()
					}
				}

			} else {
				if _, err := con.Write(src[0:data.DataLength]); err == nil {
					ret = true
				} else {
					con.Close()
				}
			}
		}
		return true
	})

	if ret {
		if montor_v, ok := t.MonitorCons.Load(serial); ok {
			if m_con, ok := montor_v.(*WrapConn); ok {
				if _, err := m_con.Write(src[0:data.DataLength]); err != nil {
					t.removeMontitor(serial)
				}
			}
		}
	}

	return ret
}

func (t *TcpServerEntity) writeDatatoCon(data DataWrapper, con *WrapConn) (bool, error) {

	ret := false
	var err error = nil
	if time.Now().Unix()-data.CreateUnixSec > t.TimeOutSec {
		log.Println("timeout:", t.TimeOutSec)
		ret = true
		return ret, err
	}

	serial := data.TargetConSerial
	src, _ := data.DataStore.Bytes()

	log.Println(t.Serial, "-->", serial, ":", string(src[0:data.DataLength]))

	if t.CryptInf != nil {

		tmp := data.DataStore.PoolPtr.GetEntity(1, 4*data.DataLength)
		defer tmp.FullRelease()

		dst, dlen := tmp.Bytes()
		status, _, dsize := t.CryptInf.EncryPt(src, dst, data.DataLength, dlen)

		if status {
			if _, err = con.Write(dst[0:dsize]); err == nil {
				ret = true
			} else {
				con.Close()
			}
		}

	} else {
		if _, err = con.Write(src[0:data.DataLength]); err == nil {
			ret = true
		} else {
			con.Close()
		}
	}

	if ret {
		if montor_v, ok := t.MonitorCons.Load(serial); ok {
			if m_con, ok := montor_v.(*WrapConn); ok {
				if _, err0 := m_con.Write(src[0:data.DataLength]); err0 != nil {
					t.removeMontitor(serial)
				}
			}
		}
	}

	return ret, err

}

//多个连接共享固定数量的conroutine进行写入
func (t *TcpServerEntity) writeDataSharePool() {
	for {
		data := t.MemQueue.SingleGet()
		if data == nil {
			continue
		}
		if dwp, ok := data.(DataWrapper); ok {
			bl := t.writeData(dwp)
			if bl || !t.NeedFeedBack {
				dwp.DataStore.ReleaseOnece()
			} else {
				t.AddDataForWrite(dwp)
			}
		}

	}
}

//每个连接占用一个conroutine进行写入
func (t *TcpServerEntity) writeDataEachConRoutine(serial string, con *WrapConn) {
	for {
		if con.IsClosed() {
			break
		}
		data := t.MemQueue.Get(serial)
		if data == nil {
			continue
		}
		if dwp, ok := data.(DataWrapper); ok {
			if dwp.DataLength < 1 {
				continue
			}
			bl, err := t.writeDatatoCon(dwp, con)
			if bl || !t.NeedFeedBack {
				dwp.DataStore.ReleaseOnece()
			} else {
				t.AddDataForWrite(dwp)
			}
			if err != nil {
				break //exit
			}
		}
	}
}

//处理新来的连接, 灵活拓展
func (t *TcpServerEntity) newComeConHandle(handle_id int) {
	buf := t.Pool.GetEntity(1, 1024)
	defer buf.FullRelease()
	for {
		if handle_id >= 0 && !t.ChanRoutineStatus[handle_id] {
			break
		}
		con := <-t.ChanQueue
		if ok, serial, tp := t.AckInf.AckSerial(con, buf, t.CryptInf); ok {
			if err := t.AckInf.AckConnect(con, serial, buf, t.CryptInf); err == nil {
				log.Println("server[", t.Serial, "] get new connect:", serial)
				WConn := &WrapConn{
					false,
					con}
				if tp == 1 {
					t.addMontitor(serial, WConn)
				} else {
					t.addTcpConn(serial, WConn)

					if t.ObserverInf != nil {
						t.ObserverInf.HNewConnect(serial, t)
						t.ObserverInf.SNewConnect(serial, buf, t, t.ToBroadCast...)
					}

					go t.createReadroutine(WConn, serial)
					if t.WriteConrutionSize < 1 {
						go t.writeDataEachConRoutine(serial, WConn)
					}

				}
			} else {
				con.Close()
			}
		} else {
			con.Close()
		}

	}
}

//启动程序
func (t *TcpServerEntity) StartListen() {
	address := ":" + t.tcpport
	log.Println(t.Serial, "tcp address", address)
	listener, err := net.Listen("tcp", address)
	t.Listener = listener
	if err != nil {
		log.Fatal("tcp server failed to start")
	}
	for i := 0; i < t.WriteConrutionSize; i++ {
		go t.writeDataSharePool()
	}
	for i := 0; i < 5; i++ {
		go t.newComeConHandle(-1) //固定5个协程
	}

	go t.createBeatSendHandle()

	index := 0
	num := 0
	sample_time := time.Now().Unix()
	for {
		if index < 0 {
			index = 0
		}
		con, err := t.Listener.Accept()
		if err != nil {
			log.Println("accept failed", err)
			continue
		}
		t.ChanQueue <- con //不采用一连接一线程处理，采用chan队列，牺牲部分效率降低开销。

		//弹性增减连接处理函数,不对ChanRoutineStatus加锁
		num = len(t.ChanQueue) / 100
		for j := 0; j < num && index < 100; j++ {
			t.ChanRoutineStatus[index] = true
			go t.newComeConHandle(index)
			index += 1
		}

		if time.Now().Unix()-sample_time > 30 {
			sample_time = time.Now().Unix()
			if index > num+1 {
				for k := num + 1; k < index && k < 100; k++ {
					t.ChanRoutineStatus[k] = false
				}
				index = num + 1
			}
		}

	}
}
