from contextlib import contextmanager
import os
import signal
import subprocess
import uuid
import socket

import requests
from retry import retry


def is_port_opened(port):
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    result = sock.connect_ex(("127.0.0.1", int(port)))
    is_opened = result == 0
    sock.close()
    return is_opened


class Server:
    def __init__(self, executable, metrics_port=8090):
        self.executable = executable
        self.process = None
        self.admin_port = 8081
        self.metrics_port = metrics_port

    def start(self):
        self.process = subprocess.Popen(
            [
                self.executable, 
                "--debug", 
                "--metrics",
                "--metrics-port", str(self.metrics_port),
                "mesh", 
                "listen"
            ],
            shell=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

        @retry(delay=0.1, tries=10 * 5)
        def _check_if_is_already_opened():
            assert is_port_opened(8080)

        _check_if_is_already_opened()
        return self

    def stop(self):
        return_code = self.process.poll()
        if return_code is None:
            return os.kill(self.process.pid, signal.SIGINT)
        stdout, stderr = self.process.communicate()
        print(stdout.decode())
        print(stderr.decode())


class Client:
    def __init__(self, executable, exposes, metrics_port=8091):
        self.executable = executable
        self.exposes = exposes
        self.metrics_port = metrics_port
        self.process = None

    def start(self):
        command = [
            self.executable, 
            "--metrics",
            "--metrics-port", str(self.metrics_port),
            "mesh", 
            "join", 
            "--name", uuid.uuid4().hex
        ]
        for expose in self.exposes:
            if type(expose) == str:
                command += ["--expose", expose]
            else:
                command += ["--expose", f"name={expose[0]},address={expose[1]}"]

        self.process = subprocess.Popen(
            command,
            shell=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        # TODO: Replace with retry once it supports multiple connections
        import time

        time.sleep(2)

        return self

    def stop(self):
        return_code = self.process.poll()
        if return_code is None:
            return os.kill(self.process.pid, signal.SIGINT)
        stdout, stderr = self.process.communicate()
        print(stdout.decode())
        print(stderr.decode())


class MockServer:
    def __init__(self, executable, port=1234, response=None):
        self.executable = executable
        self.process = None
        self.port = port
        self.response = response

    def start(self):
        self.process = subprocess.Popen(
            [
                self.executable, "testserver", "--port", str(self.port)
            ] + (['--response', self.response] if self.response else []),
            shell=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

        @retry(delay=0.1, tries=10 * 5)
        def _check_if_is_already_opened():
            assert is_port_opened(self.port)

        _check_if_is_already_opened()
        return self

    def stop(self):
        return_code = self.process.poll()
        if return_code is None:
            return os.kill(self.process.pid, signal.SIGINT)
        stdout, stderr = self.process.communicate()
        print(stdout.decode())
        print(stderr.decode())


@contextmanager
def launched_in_background(process):
    try:
        process.start()
        yield process
    finally:
        process.stop()


def get_number_of_running_goroutines(port=8090):
    return int(
        [
            l for l in requests.get(
                f'http://localhost:{port}/metrics'
            ).text.split('\n') if l.strip().startswith('go_goroutines')
        ][0].split(' ')[1]
    )
