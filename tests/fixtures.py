import json
import os
import shutil
import signal
import socket
import subprocess
import uuid
from contextlib import contextmanager

import psutil
import pymysql
import requests
from retry import retry


def is_port_opened(port):
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    result = sock.connect_ex(("127.0.0.1", int(port)))
    is_opened = result == 0
    sock.close()
    return is_opened


def run_process(command, *args, **kwargs):
    print(f">>> {' '.join(command)}")
    return subprocess.run(command, *args, **kwargs)


class Server:
    def __init__(self, executable, metrics_port=8090, acceptor="dummy"):
        self.executable = executable
        self.process = None
        self.admin_port = 8081
        self.metrics_port = metrics_port
        self.acceptor = acceptor

    def start(self):
        self.process = subprocess.Popen(
            [
                self.executable,
                "--debug",
                "--metrics",
                "--metrics-port",
                str(self.metrics_port),
                "mesh",
                "listen",
                "--acceptor",
                self.acceptor,
            ],
            shell=False,
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

    def admin(self, path):
        return f'http://localhost:{self.admin_port}/{path if not path.startswith("/") else path[1:]}'


class MySQLServer:
    def __init__(self):
        self.container_id = f"mysql-{uuid.uuid4().hex}"
        self.host = "localhost"
        self.port = 3306
        self.user = "root"
        self.password = "123123"

    def start(self):
        process = run_process(
            [
                "docker",
                "run",
                "--rm",
                "-d",
                "--network=host",
                "--name",
                self.container_id,
                "-e",
                f"MYSQL_ROOT_PASSWORD={self.password}",
                "mysql:latest",
            ],
            shell=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=True,
        )

        self.container_id = process.stdout.decode().strip()

        @retry(delay=2, tries=120)
        def _check_if_mysql_already_listens():
            pymysql.connect(host=self.host, user=self.user, password=self.password)

        _check_if_mysql_already_listens()

    def stop(self):
        run_process(
            ["docker", "rm", "-f", self.container_id],
            shell=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=True,
        )


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
            "--metrics-port",
            str(self.metrics_port),
            "mesh",
            "join",
            "--name",
            uuid.uuid4().hex,
        ]
        for expose in self.exposes:
            if type(expose) == str:
                command += ["--expose", expose]
            else:
                command += ["--expose", f"name={expose[0]},address={expose[1]}"]

        self.process = subprocess.Popen(command, shell=False)
        # TODO: Replace with retry once it supports multiple connections
        import time

        time.sleep(2)

        return self

    def stop(self):
        return_code = self.process.poll()
        if return_code is None:
            return os.kill(self.process.pid, signal.SIGINT)


class MockServer:
    def __init__(self, executable, port=1234, response=None):
        self.executable = executable
        self.process = None
        self.port = port
        self.response = response

    def start(self):
        self.process = subprocess.Popen(
            [self.executable, "testserver", "--port", str(self.port)]
            + (["--response", self.response] if self.response else []),
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

    def endpoint(self):
        return f"localhost:{self.port}"


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
            metrics
            for metrics in requests.get(f"http://localhost:{port}/metrics").text.split(
                "\n"
            )
            if metrics.strip().startswith("go_goroutines")
        ][0].split(" ")[1]
    )


def get_number_of_opened_files(process_owner):
    return len(psutil.Process(pid=process_owner.process.pid).open_files())


class Kubectl:
    def __init__(self, cluster):
        self.cluster = cluster

    def json(self, command):
        return json.loads(self.run(command + ["-o", "json"]).stdout.decode())

    def run(self, command):
        return run_process(
            [self.executable(), "--kubeconfig", self.cluster.kubeconfig] + command,
            shell=False,
            stdout=subprocess.PIPE,
            check=True,
            cwd=os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
        )

    @classmethod
    def executable(cls):
        if shutil.which("kubectl"):
            return shutil.which("kubectl")
        if os.path.isfile("/tmp/kubectl"):
            return "/tmp/kubectl"

        latest_version = requests.get(
            "https://storage.googleapis.com/kubernetes-release/release/stable.txt"
        ).text
        download(
            f"https://storage.googleapis.com/kubernetes-release/release/{latest_version}"
            "/bin/linux/amd64/kubectl",
            "/tmp/kubectl",
        )
        run_process(["chmod", "+x", "/tmp/kubectl"], check=True)
        return "/tmp/kubectl"


class KindCluster:

    KIND_VERSION = "v0.11.1"

    def __init__(self, name):
        self.name = name
        self.kubeconfig = os.path.join("/tmp", f"kind-{self.name}-kubeconfig")

    @property
    def exists(self):
        result = run_process(
            ["docker", "ps", "--format", "{{ .Names }}"], stdout=subprocess.PIPE
        )
        assert not result.returncode, "Could not list running docker containers"
        exists = f"{self.name}-control-plane" in result.stdout.decode()
        return exists

    def create(self):
        assert not self.exists, f"Cannot create cluster {self.name} - it already exists"
        assert not run_process(
            [
                self.executable(),
                "create",
                "cluster",
                "--name",
                self.name,
                "--kubeconfig",
                self.kubeconfig,
            ],
        ).returncode, "Could not create cluster"

        @retry(tries=60, delay=2)
        def wait_for_cluster_availability():
            Kubectl(self).run(["get", "namespaces"])

        wait_for_cluster_availability()

    def delete(self):
        assert self.exists, f"Cannot delete cluster {self.name} - it does not exist"
        assert not run_process(
            [self.executable(), "delete", "cluster", "--name", self.name],
        ).returncode, "Could not delete cluster"

    def executable(self):
        if shutil.which("kind"):
            return shutil.which("kind")
        if os.path.isfile("/tmp/kind-linux-amd64"):
            return "/tmp/kind-linux-amd64"
        download(
            f"https://github.com/kubernetes-sigs/kind/releases/download/{self.KIND_VERSION}/kind-linux-amd64",
            "/tmp/kind-linux-amd64",
        )
        assert not run_process(["chmod", "+x", "/tmp/kind-linux-amd64"]).returncode
        return "/tmp/kind-linux-amd64"

    def load_image(self, image):
        run_process(
            [self.executable(), "load", "docker-image", "--name", self.name, image],
            check=True,
        )


class Helm:

    HELM_VERSION = "v3.8.2"

    def __init__(self, cluster):
        self.cluster = cluster

    def install(self, name, values, namespace=None):
        self.run(
            [
                "install",
                "-n",
                namespace or name,
                name,
                "kubernetes/helm",
                "--wait",
                "--set",
                "client.pullPolicy=Never",
                "--set",
                "server.pullPolicy=Never",
                "--set",
                "docker.version=pytest",
            ]
            + [
                item
                for sublist in [["--set", f"{k}={v}"] for k, v in values.items()]
                for item in sublist
            ]
        )

    def run(self, command):
        process = run_process(
            [self.executable(), "--kubeconfig", self.cluster.kubeconfig] + command,
            shell=False,
            stdout=subprocess.PIPE,
            check=True,
            cwd=os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
        )
        return process

    def executable(self):
        if shutil.which("helm"):
            return shutil.which("helm")
        final_extract_path = "/tmp/linux-amd64/helm"
        if os.path.isfile(final_extract_path):
            return final_extract_path
        download(
            f"https://get.helm.sh/helm-{self.HELM_VERSION}-linux-amd64.tar.gz",
            f"/tmp/helm-{self.HELM_VERSION}-linux-amd64.tar.gz",
        )
        assert not run_process(
            ["tar", "-xvzf", f"helm-{self.HELM_VERSION}-linux-amd64.tar.gz"], cwd="/tmp"
        ).returncode
        return final_extract_path


def download(url, path):
    response = requests.get(url, allow_redirects=True)
    assert response.status_code < 299, f"Could not download file from {url}"
    with open(path, "wb") as f:
        f.write(response.content)
