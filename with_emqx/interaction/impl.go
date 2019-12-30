package interaction

import (
	"net"
)

//连接接口
type ConImp interface {
	net.Conn                 //接口
	SetSerial(serial string) //设置连接序列号[连接所控制设备/主机的序列号]
	PeerAddress() string     //远程主机地址
	Serial() string          //读取序列号
	Ok() bool                //连接是否可用
}

//服务接口
type ServerImp interface {
	ClientIsOnline(serial string) bool  //服务下控制 序列号为serial的主机的连接 是否存在
	ClientSerial() string               //读取服务名
	Publish(serial string, data []byte) //消息推送
	StartListen()                       //开始监听
}
