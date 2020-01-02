# -*- coding:utf-8 -*-

import websockets
import asyncio
import ssl
from log import Logger
from typing import Callable
from servers.utils import ConDict


class WsServer:
    def __init__(self, host, port, cons: ConDict = None, mqtt_publish: Callable[[str, str], None] = None,  crt=None, key=None):
        """
        :param host: host
        :param port: 端口
        :param path: 路径
        :param crt:  证书私钥
        :param key:  证书公钥
        """
        self.conns = cons
        self.host = host
        self.port = port
        self.mqtt_publish = mqtt_publish
        if crt and key:
            self.ssl = ssl.SSLContext(protocol=ssl.PROTOCOL_TLS)
            self.ssl.load_cert_chain(certfile=crt, keyfile=key)
        else:
            self.ssl = None

    @staticmethod
    def address_to_string(addr):
        return addr[0] + ":%i" % addr[1]

    @staticmethod
    async def auth(ws)->(bool, str):
        try:
            msg = await ws.recv()
        except Exception as e:
            Logger.warn(e)
            return False, ""
        if msg.startswith("TTG"):
            await ws.send("WEB_ACCEPT")  # 可不捕获，整个handle都在try里面
            return True, msg
        return False, ""

    async def hold_connect(self, ws, serial: str):
        while True:
            try:
                msg = await ws.recv()
            except Exception as e:
                Logger.warning(e)
                break
            if self.mqtt_publish is not None:
                self.mqtt_publish(serial, msg)

    async def handle(self, ws, _):
        bl, serial = await self.auth(ws)
        if not bl:
            return
        address = self.address_to_string(ws.remote_address)
        if self.conns is not None:
            if not self.conns.add_con(serial, address,  ws):
                return
        await self.hold_connect(ws, serial)
        if self.conns is not None:
            self.conns.remove_con(serial, address)

    def listen(self):
        if self.ssl is not None:
            server = websockets.serve(ws_handler=self.handle,
                                      host=self.host,
                                      port=self.port,
                                      ssl=self.ssl
                                      )
        else:
            server = websockets.serve(ws_handler=self.handle,
                                      host=self.host,
                                      port=self.port
                                      )
        asyncio.get_event_loop().run_until_complete(server)
        asyncio.get_event_loop().run_forever()  # pending


if __name__ == '__main__':
    Sv = WsServer("127.0.0.1", 8099)
    Sv.listen()

