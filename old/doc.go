// CloudServer project doc.go

/*
CloudServer document

云端.控制服务端

auth: TTG
date:2019/7/15


1.redis保存连接状态， 对于数据量较少，redis还是相对比较稳定
2.本地log,保存warning错误
3.可配置
-*. app tcp端口
_*. 其它待连接信息tcp端口
_*. 家具控制端tcp端口
_*. redis host
_*. redis port
_*. redis auth
_*. 心跳包数据



*/

package main
