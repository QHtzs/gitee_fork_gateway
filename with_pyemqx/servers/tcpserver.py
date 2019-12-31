# -*- coding:utf-8 -*-

"""
1.windows下python没有完全端口api，对并发支持不大
2.python socket库未对相关操作提供async操作，实质还是阻塞
"""

import socketserver
from servers.utils import ConDict
from typing import Callable


class TcpServerHandle(socketserver.BaseRequestHandler):
    def __init__(self, cons: ConDict = None, mqtt_publish: Callable[[str, str], None] = None):
        super().__init__()
        self.cons = cons
        self.mqtt_publish = mqtt_publish




TcpServerHandle()










