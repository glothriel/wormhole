import logging
import os
import socket
import subprocess
import tempfile

import pytest

from .fixtures import Client, MockServer, Server, MySQLServer

logger = logging.getLogger(__name__)

TEST_SERVER_PORT = 1234


def is_port_opened(port):
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    result = sock.connect_ex(("127.0.0.1", int(port)))
    is_opened = result == 0
    sock.close()
    return is_opened


def run_process(process, **kwargs):
    logger.info(" ".join(process))
    rt = subprocess.run(
        process, stdout=subprocess.PIPE, stderr=subprocess.PIPE, **kwargs
    )
    try:
        rt.check_returncode()
    finally:
        logger.info(rt.stdout.decode())
        logger.info(f"Return code: {rt.returncode}")
        stderr = rt.stderr.decode()
        if stderr:
            logger.error(stderr)
    return rt


@pytest.fixture(scope="session")
def executable():
    tmp = None
    try:
        tmp = tempfile.NamedTemporaryFile(prefix="wormhole", delete=False)
        run_process(["go", "build", "-o", tmp.name, "main.go"])
        yield tmp.name
    finally:
        os.unlink(tmp.name)


@pytest.fixture()
def client(executable, server):
    c = Client(executable, exposes=[f"localhost:{TEST_SERVER_PORT}"])
    try:
        yield c.start()
    finally:
        c.stop()


@pytest.fixture()
def server(executable):
    server = Server(executable)
    try:
        yield server.start()
    finally:
        server.stop()


@pytest.fixture()
def mysql():
    mysql = MySQLServer()
    try:
        mysql.start()
        yield mysql
    finally:
        mysql.stop()


@pytest.fixture()
def mock_server(executable):
    server = MockServer(executable)
    try:
        yield server.start()
    finally:
        server.stop()
