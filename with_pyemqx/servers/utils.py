# -*- coding:utf-8 -*-

import threading
from typing import Callable, Any
from log import Logger


class ConDict:
    def __init__(self, serial, sid, close_func: Callable[[Any], None]):
        self.m_serial = serial
        self.m_id = sid
        self.contain = dict()
        self.close = close_func
        self._mutex = threading.Lock()
        self.new_con = None
        self.dis_con = None

    def set_action(self, new_con: Callable[[str, str], None], dis_con: Callable[[str, str], None]):
        self.new_con = new_con
        self.dis_con = dis_con

    def add_con(self, serial, peer_addr, con)->bool:
        with self._mutex:
            d = self.contain.get(serial, None)
            if d is None:
                self.contain[serial] = dict()
            elif d.get(peer_addr, None) is not None:
                Logger.info("con already existss")
                return False
        self.contain[serial][peer_addr] = con
        self.new_con(serial, peer_addr) # new_con含self._mutex锁，请勿在_mutex下,
        return True

    def remove_con(self, serial, peer_addr):
        with self._mutex:
            d = self.contain.get(serial, None)
            if d is None:
                return
            con = d.get(peer_addr, None)
            if con is not None:
                self.contain[serial].pop(peer_addr)
            else:
                return
        self.dis_con(serial, peer_addr)
        self.close(con)

    def serial_exists(self, serial)->bool:
        with self._mutex:
            return True if self.contain.get(serial, {}) else False

    def publish(self, serial, msg, func: Callable[[Any, str], None]):
        with self._mutex:
            cons = self.contain.get(serial, {}).values()
        for con in cons:
            func(con, msg)

    def all_conect(self):
        cons = []
        with self._mutex:
            for d in self.contain.values():
                for v in d:
                    cons.extend(v.values())
        return cons