# docker
* 建议Linux系统，windows下docker不大友好
* windows：对于内存2G以上的系统支持
* linux：ubuntu: apt-get install docker*;其它系统安装也简单，可搜教程

# EMQ（非商用版）
* EMQ集成了MQTT(tcp), MQTT-SN(udp), LWM2M协议。
* EMQ具备较丰富的插件支持，插件使用配置简单
* 号称普通版十万tcp单机长连接支持,商用百万并发,商用平台千万并发
* 支持集群的形式部署
* 商用版在数据持久化具备更好的支持
* 与其他的MQ中间件等可友好对接
* 一些其它机制及未来拓展机制

# EMQ简单测试
* 受设备限制，暂未进行压测。
* 少量客户端连接(与broker同网卡),测试结果为topic从publish到被其它相关客户端完成订阅,<br>
时间为100纳秒以内，具备较高的实时性
* 内存问题，emqx_retainer插件按需配置,否则内存爆炸。内存池化，回收速度较慢

#  EMQ部署
* docker pull emqx/emqx
* docker run -d --name emqx -p 1883:1883 -p 8883:8883 -p8083:8083 
-p 8084:8084 -p 8080:8080 -p 18083:18083 emqx/emqx:latest <br>

* 需要做端口映射的端口 <br>
1883	MQTT 协议端口 <br>
8883	MQTT/SSL 端口 <br>
8083	MQTT/WebSocket 端口 <br>
8084	MQTT/WebSocket/SSL 端口 <br>
8080	管理API 端口 <br>
18083	Dashboard 端口 <br>

* 进入dashboard: http://宿主ip:宿主映射容器18083的端口 &nbsp;进行配置
* 为提高安全性,请配置[登录授权](https://docs.emqx.io/broker/v3/cn/plugins.html#mongodb),采用mongodb表做鉴权

#  设备状态持久化
## 数据库暂时使用mongodb
* mongodb支持集群方案，redis集群暂不友好
* mongodb在数据持久化上优于redis
* mongodb在update操作上，优于sql型数据库。对比其它nosql数据库:
在update上效率优于es且稳定,在数据友好方面等优于hive。

## mongodb安装
* docker pull mongo 
* docker run --name=mongo -d -p 27017:27017 mongo:latest --auth
* //mongodb默认开放27017端口, --auth参数为需要验证。建议-v对数据存储位置做地址映射(-v 宿主盘某位置:/data/db)
* 登录mongo,进行账户配置。(当默认账户个数为0时,哪怕需鉴权，无验证状态下的连接也可以操作mongo)
* 创建相关表

# 硬件需求
* mongodb为内存型数据库，需较多的内存。索引较多时所需内存往往是所存储数据的几倍，才能发挥出较好性能
* EMQ类似，并发较高，队列中数据较多也需要较高的内存。
* 具体内存需求，建议部署时采用2核8G或更高配置的服务器【测试可4G内存】

# 镜像
* 鉴于 EMQ和mongodb 在docker hub上均有镜像且配置简单,暂不自己封装镜像

------------------------------------------

# 结构
* 其它控制入口(http/ws/tcp等)<-->{网关控制器(xxx + mqtt client)<--->EMQ(mqtt broker)<-->}[主机，设备等]（mqtt client)
* 其中{}中内容为早前开发的服务，现在采用EMQ达到解耦目的. xxx为动态服务：可能是ws或者http，tcp等一个或多个服务
* 网关控制器中处理数据交互<br>

1. 状态持久化(保存设备状及其他状态到mongodb)<br>
2. 指令交互(指令通过mqttclient[A1] publish到broker并监听/订阅守候回复, broker把消息推送到相关设备或者主机，主机接到指令并处理后发布相关topic到broker，broker将信息发送给订阅者[包括A1])

# 暂定TOPIC规则
* 根节点 项目名或者项目方公司名 如TTG,DPRO
* 根节点之后节点，根据需求定

----------------------------------
# 针对本项目
## I 设计，沿用早前结构暂不做优化

1. 设备/主机登录登出消息主题 TTG/ONLINE,消息内容:hostid string,请设置遗愿消息针对异常断开连接.<br>
/* <br>
MQTT遗愿消息(Last Will)<br>
MQTT客户端向服务器端CONNECT请求时，<br>
可以设置是否发送遗愿消息(Will Message)标志，<br>
和遗愿消息主题(Topic)与内容(Payload)。<br>
MQTT客户端异常下线时(客户端断开前未向服务器发送DISCONNECT消息)，<br>
MQTT消息服务器会发布遗愿消息。<br>
*/<br>

2. 设置控制 TTG/GATEWAY/`DEVICEID`,`DEVICEID`为设备的id或者主机ID。内容照之前格式 

3. 网关与外接口客户端特殊化不遵循TTG/GATEWAY/`DEVICEID`采用独立topic  TTG/CONTROL/`DEVICEID`，避免监听到自己pub的消息，导致没必要的资源消耗。

4. 设备与Broker连接时，clientID最好设置为`DEVICEID`的值，且根据需求采取session连接方式 <br>
/*<br>
 MQTT客户端向服务器发起CONNECT请求时，可以通过’Clean Session’标志设置会话。<br>
‘Clean Session’设置为0，表示创建一个持久会话，在客户端断开连接时，会话仍然保持并保存离线消息，直到会话超时注销。<br>
‘Clean Session’设置为1，表示创建一个新的临时会话，在客户端断开时，会话自动销毁。<br>
*/<br>

## 根据 I 则订阅和发布遵循基本规则
CONTROL:  pub:[TTG/GATEWAY/`DEVICEID`], subscribe:[TTG/CONTROL/#]  <br>
HOST_OR_DEVICE: pub:[TTG/CONTROL/`DEVICEID`], subscribe:[TTG/GATEWAY/`DEVICEID`] <br>

##  结构
* [类UML图](mx.png)
* [UML图](uml.png)








