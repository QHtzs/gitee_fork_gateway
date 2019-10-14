package main

import (
	"net"
)

//心跳接口
type BeatPackageImpl interface {
	BeatBytes() []byte    //心跳包
	BeatAckBytes() []byte //心跳确认包
	BeatInteval() int64   //心跳发送频率
}

//加密解密接口
type CryptImpl interface {
	EncryPt(src, dst []byte, src_len, dst_buff_len int) (bool, int, int) // 是否成功, 加密后src中有效byte量, 解析后dst中有效byte量
	DeCrypt(src, dst []byte, src_len, dst_buff_len int) (bool, int, int) // 是否成功, 解析后src中有效byte量, 解析后dst中有效byte量
}

//连接状态观察
type ConChangeObserverImpl interface {
	SNewConnect(serial string, entity *MemEntity, v ServerImpl, vs ...ServerImpl) //新连接,反馈给Tcp/udp server
	SDisConnect(serial string, entity *MemEntity, v ServerImpl, vs ...ServerImpl) //连接断开,反馈给Tcp/udp server
	HNewConnect(serial string)                                                    //新连接,  反馈给http
	HDisConnect(serial string)                                                    //连接断开，反馈给http
}

//解包器,并返回业务处理后需要反馈结果到toself, tocast发送给自己，转发给其它。 返回byte长度
type PackageParseImpl interface {
	Parser(server_serial, cur_con_serial string, src, toself, tocast *MemEntity, src_len *int, v CryptImpl) (s_size int, c_size int, serial string, beat bool) //解包器,并返回业务处理后需要反馈结果到toself, tocast发送给自己，转发给其它。 返回byte长度
}

//认证
type AckImpl interface {
	AckSerial(con net.Conn, buf *MemEntity, v CryptImpl) (bool, string, int)   //获取serial并简单认证, bool:认证状态, serial:认证id, int:serial类型1:监控,非1：普通tcp
	AckConnect(con net.Conn, serial string, buf *MemEntity, v CryptImpl) error //简单握手规则
}

//服务接口
type ServerImpl interface {
	AddToDistributeEntity(v ServerImpl)  //对象关联, v为接收句柄对象,以及数据交互是否加密
	SerialIsActivity(serial string) bool //判断socket是否连接
	GetSerial() string                   //获取序列号
	AddDataForWrite(data DataWrapper)    //写入数据转发队列
	BroadCastData(data DataWrapper)      //广播数据
}

//数据封装,用于存放进chan
type DataWrapper struct {
	DataStore       *MemEntity   //数据存在地址
	DataLength      int          //数据长度
	TargetConSerial string       //目标con的serial
	UdpAddr         *net.UDPAddr //当为udp时UdpAddr为serial
	SelfId          string       //to self
	CreateUnixSec   int64        //创建时间
}
