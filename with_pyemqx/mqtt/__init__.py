# -*- coding:utf-8 -*-
from log import Logger

import paho.mqtt.client as mqtt


__all__ = ["create_mqtt_client"]


def on_connect(client, userdata, flags, rc):
    Logger.info("connected: client id:%s; result code: %s" % (userdata, rc) )


def create_mqtt_client(host, client_id, username, password):
    client = mqtt.Client(client_id=client_id, clean_session=True, userdata=client_id)
    client.on_connect = on_connect
    client.username_pw_set(username=username, password=password)
    client.will_set("TTG/ONLINE", "OFF", 0)
    client.connect(host)
