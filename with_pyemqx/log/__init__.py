# -*- coding:utf-8 -*-

import logging


__all__ = ["Logger"]


def create_file_logger(file_name: str):
    logger = logging.getLogger(__name__)
    logger.setLevel(level=logging.INFO)
    handler = logging.FileHandler(file_name)
    handler.setLevel(level=logging.INFO)
    fmt = logging.Formatter("%(asctime)s - [line:%(lineno)s  level:%(levelname)s]  %(message)s")
    c_handle = logging.StreamHandler()
    c_handle.setLevel(level=logging.INFO)
    handler.setFormatter(fmt)
    c_handle.setFormatter(fmt)
    logger.addHandler(handler)
    logger.addHandler(c_handle)
    return logger


Logger = create_file_logger("log.log")


if __name__ == '__main__':
    Logger.warning("asdasd")