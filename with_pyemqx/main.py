# -*- coding:utf-8 -*-

from servers.tcpserver import start_tcp_listen
from servers.wss import WsServer
from servers.utils import ConDict
from mqtt import create_mqtt_client
from threading import Thread
import config
import requests
import re
from functools import partial


def new_host_connected(status, serial):
    url = config.URL + "serial=%s&gateway_status=%i" % (serial, status)
    try:
        requests.get(url, timeout=(2, 2))
    except:
        pass


def create_on_message(*cons):
    """接收mqtt发来的消息，并广播到app/web"""
    def on_message(client, userdata, message):
        serial = re.findall("[^/]+$", message.topic)
        if message.payload == "0":
            on_host_offline(serial)
        elif message.payload == "1":
            on_host_online(serial)
        else:
            msg = message.payload
            msg = msg if isinstance(msg, str) else msg.decode()
            for con in cons:
                if con.m_id == "tcp":
                    con.publish(serial, msg, lambda x, y: x.send(y.encode()))
                if con.m_id == "ws":
                    con.publish(serial, msg, lambda x, y: x.send(y))

    return on_message


def create_on_new_connect(client, *cons):
    def new_connect(serial, peername):
        d = dict()
        for con in cons:
            d[con.m_serial] = con.serial_exists(serial)
        msg = NONE_UPDATE
        if d["WEB"] and d["APP"]:
            msg = ALL_UPDATE
        elif d["WEB"]:
            msg = WEB_UPDATE
        elif d["APP"]:
            msg = APP_UPDATE
        client.publish("TTG/GATEWAY/%s" % serial, msg)

    return new_connect


def mqtt_publisher(client, serial, msg):
    client.publish("TTG/CONTROL/%s" % serial, msg)


WEB_UPDATE = '{"Gate_UPDATE": {"mode": "web" }}'
APP_UPDATE = '{"Gate_UPDATE": {"mode": "app" }}'
ALL_UPDATE = '{"Gate_UPDATE": {"mode": "all" }}'
NONE_UPDATE = '{"Gate_UPDATE": {"mode": "none"}}'
TcpCon0 = ConDict("APP", "tcp", lambda x: x)
TcpCon1 = ConDict("WEB", "tcp", lambda x: x)
WssCon = ConDict("WEB", "ws", lambda x: x.close())
on_host_online = partial(new_host_connected, 1)
on_host_offline = partial(new_host_connected, 0)
# 涉及到全局性相互引用，生存周期为整个程序运行过程，故不采用weakref
# 便于理解，采用代码平铺式
client = create_mqtt_client(config.Mqtt["host"], "con_client", config.Mqtt["user"], config.Mqtt["pwd"])
on_new_connect = create_on_new_connect(client, TcpCon0, TcpCon1, WssCon)
TcpCon0.set_action(new_con=on_new_connect, dis_con=on_new_connect)
TcpCon1.set_action(new_con=on_new_connect, dis_con=on_new_connect)
WssCon.set_action(new_con=on_new_connect, dis_con=on_new_connect)
client2 = create_mqtt_client(config.Mqtt["host"], "pub_sub_client", config.Mqtt["user"], config.Mqtt["pwd"])
client2.on_message = create_on_message(TcpCon0, TcpCon1, WssCon)
client2.subscribe("TTG/GATEWAY/#")
mqtt_publish = partial(mqtt_publisher, client2)

if __name__ == '__main__':
    # 变量均为线程级的共享，请勿多进程
    t = Thread(target=start_tcp_listen, args=(8385, TcpCon0,  mqtt_publish))
    t.setDaemon(True)
    t.start()

    t = Thread(target=start_tcp_listen, args=(8386, TcpCon1,  mqtt_publish))
    t.setDaemon(True)
    t.start()

    s = WsServer("0.0.0.0", 8384, WssCon, mqtt_publish)
    s.listen()
