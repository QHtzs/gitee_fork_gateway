# -*- coding:utf-8 -*-

"""
1.windows下python没有完全端口api，对并发支持不大
2.python socket库未对相关操作提供async操作，实质还是阻塞
3.采用封装好的框架socketserver
"""

import socketserver
from servers.utils import ConDict
from typing import Callable
from log import Logger


class TcpServerHandle(socketserver.BaseRequestHandler):
    cons = None
    mqtt_publish = None

    def __init__(self, request, client_address, server):
        super().__init__(request, client_address, server)
        self.m_serial = ""
        self.m_address = ""
        self.m_add = False

    @staticmethod
    def address_to_string(addr):
        return addr[0] + ":%i" % addr[1]

    def handle(self):
        self.m_add = False
        try:
            msg = self.request.recv(1024)
            if msg.startswith(b"TTG"):
                self.m_address = self.address_to_string(self.client_address)
                self.m_serial = msg.decode()
                if self.cons is not None:
                    if self.cons.add_con(self.m_serial, self.m_address, self.request):
                        self.m_add = True
                        self.request.send(b"WEB_ACCEPT")
                        while True:
                            text = self.request.recv(1024)
                            text = text.decode()
                            if self.mqtt_publish is not None:
                                self.mqtt_publish(self.m_serial, text)
                    else:
                        self.request.send(b"con exists")
        except Exception as e:
            Logger.warn(e)
        finally:
            self.request.close()

    def finish(self):
        if self.cons is not None:
            if self.m_add:
                self.cons.remove_con(self.m_serial, self.m_address)


def start_tcp_listen(port, cons: ConDict, mqtt_publish: Callable[[str, str], None]):
    """warn：局部改变类变量会上升到全局，请谨慎操作 """
    TcpServerHandle.cons = cons
    TcpServerHandle.mqtt_publish = mqtt_publish
    server = socketserver.TCPServer(("0.0.0.0", port), TcpServerHandle)
    server.serve_forever()


if __name__ == '__main__':
    start_tcp_listen(8088, None, None)
