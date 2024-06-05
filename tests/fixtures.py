import json
import os
import shutil
import signal
import subprocess
import socket
import uuid
from contextlib import contextmanager
import tempfile

import psutil
import pymysql
import requests
from retry import retry


def run_process(command, *args, **kwargs):
    print(f">>> {' '.join(command)}")
    return subprocess.run(command, *args, **kwargs)


class Server:
    def __init__(
        self,
        executable,
        state_manager_path="/tmp/server-state-manager",
        nginx_confd_path="/tmp/server-nginx-confd",
        wireguard_config_path="/tmp/server-wireguard/wg0.conf",
        wireguard_address="0.0.0.0",
        wireguard_subnet="24",
        metrics_port=8090,
    ):
        self.executable = executable
        self.state_manager_path = state_manager_path
        self.nginx_confd_path = nginx_confd_path
        self.wireguard_config_path = wireguard_config_path
        self.metrics_port = metrics_port
        self.wireguard_address = wireguard_address
        self.wireguard_subnet = wireguard_subnet
        self.process = None

    def start(self):
        cmd = [
            self.executable,
            "--debug",
            "--metrics",
            "--metrics-port",
            str(self.metrics_port),
            "server",
            "--name",
            uuid.uuid4().hex,
            "--directory-state-manager-path",
            self.state_manager_path,
            "--nginx-confd-path",
            self.nginx_confd_path,
            "--wg-config",
            self.wireguard_config_path,
            "--wg-public-host",
            self.wireguard_address,
            "--wg-internal-host",
            self.wireguard_address,
            "--wg-subnet-mask",
            self.wireguard_subnet,
            "--invite-token",
            "123123",
        ]
        print(" ".join([str(i) for i in cmd]))
        self.process = subprocess.Popen(
            cmd,
            shell=False,
        )

        @retry(delay=0.1, tries=50)
        def _check_if_is_already_opened():
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
                try:
                    s.connect(('localhost', self.metrics_port))
                    return True
                except (ConnectionRefusedError, OSError):
                    raise Exception("Port is not open yet")

        _check_if_is_already_opened()

        return self

    def stop(self):
        return_code = self.process.poll()
        if return_code is None:
            return os.kill(self.process.pid, signal.SIGINT)

    def admin(self, path):
        return (
            f'http://localhost:{self.admin_port}/{path if not path.startswith("/") else path[1:]}'
        )


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
    def __init__(
        self,
        executable,
        server,
        state_manager_path="/tmp/client-state-manager",
        nginx_confd_path="/tmp/client-nginx-confd",
        wireguard_config_path="/tmp/client-wireguard/wg0.conf",
        metrics_port=8091,
    ):
        self.executable = executable
        self.server = server
        self.state_manager_path = state_manager_path
        self.nginx_confd_path = nginx_confd_path
        self.wireguard_config_path = wireguard_config_path
        self.metrics_port = metrics_port
        self.process = None

    def start(self):
        command = [
            self.executable,
            "--metrics",
            "--metrics-port",
            str(self.metrics_port),
            "client",
            "--name",
            uuid.uuid4().hex,
            "--nginx-confd-path",
            self.nginx_confd_path,
            "--wg-config",
            self.wireguard_config_path,
            "--directory-state-manager-path",
            self.state_manager_path,
            "--invite-token",
            "123123",
        ]
        self.process = subprocess.Popen(command, shell=False)
        return self

    def stop(self):
        return_code = self.process.poll()
        if return_code is None:
            return os.kill(self.process.pid, signal.SIGINT)


class MockServer:
    def __init__(self, kubectl, wormhole_image):
        self.namespace = "nginx"
        self.name = "nginx"
        self.kubectl = kubectl

    def start(self):
        self.kubectl.run(["create", "ns", self.namespace])

        @retry(tries=20, delay=.5)
        def _wait_for_mocks():
            self.kubectl.run(["apply", "-f", "kubernetes/raw/mocks/all.yaml"])
        _wait_for_mocks()
        return self

    def stop(self):
        self.kubectl.run(["delete", "ns", self.namespace])


class Curl:
    def __init__(self, kubectl):
        self.kubectl = kubectl

    def start(self):

        @retry(tries=20, delay=.5)
        def _wait_for_mocks():
            self.kubectl.run(["apply", "-f", "kubernetes/raw/curl/all.yaml"])
        _wait_for_mocks()
        return self

    def stop(self):
        self.kubectl.run(["delete", "-f", "kubernetes/raw/curl/all.yaml"])

    def call_with_network_policy(self, command, max_time_seconds=None):
        return self._call('curl-with-labels', command, max_time_seconds)

    def call_without_network_policy(self, command, max_time_seconds=None):
        return self._call('curl-no-labels', command, max_time_seconds)

    def _call(self, pod, command, max_time_seconds=None):
        max_time_seconds = max_time_seconds or 20
        return self.kubectl.run(
            [
                '-n',
                'default',
                'exec',
                f'pod/{pod}',
                '--',
                'curl',
                '-m', str(max_time_seconds),
            ] + (command if type(command) is list else [command])
        )


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
            for metrics in requests.get(f"http://localhost:{port}/metrics").text.split("\n")
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

    KIND_VERSION = "v0.23.0"

    def __init__(self, name):
        self.name = name
        self.existed_before = False
        self.kubeconfig = os.path.join("/tmp", f"kind-{self.name}-kubeconfig")

    @property
    def exists(self):
        result = run_process(["docker", "ps", "--format", "{{ .Names }}"], stdout=subprocess.PIPE)
        assert not result.returncode, "Could not list running docker containers"
        exists = f"{self.name}-control-plane" in result.stdout.decode()
        return exists

    def create(self):
        if self.exists:
            self.existed_before = True
            return
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

        Kubectl(self).run(['create', '-f', 'kubernetes/raw/kind/nps.yaml'])

    def delete(self):
        assert self.exists, f"Cannot delete cluster {self.name} - it does not exist"
        if self.existed_before or json.loads(os.getenv("REUSE_CLUSTER") or 'false'):
            print("Skipping removal of KIND cluster - it existed before the tests were run")
            return
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


class K3dCluster:

    K3D_VERSION = "v5.6.3"

    def __init__(self, name):
        self.name = name
        self.existed_before = False
        self.kubeconfig = os.path.join("/tmp", f"kind-{self.name}-kubeconfig")

    @property
    def exists(self):
        result = self.run(["cluster", "list", "-o", "json"])
        clusters = json.loads(result.stdout.decode())
        return self.name in [cluster["name"] for cluster in clusters]

    def create(self):
        if self.exists:
            self.existed_before = False
            return
        self.run(
            [
                "cluster",
                "create",
                self.name,
                "--registry-create",
                self.name,
            ],
        )

        kubeconfig = self.run(
            [
                "kubeconfig",
                "get",
                self.name,
            ],
        ).stdout.decode()

        with open(self.kubeconfig, "w") as f:
            f.write(kubeconfig)

        @retry(tries=60, delay=2)
        def wait_for_cluster_availability():
            Kubectl(self).run(["get", "namespaces"])

        wait_for_cluster_availability()

    def delete(self):
        if not self.exists:
            return
        if self.existed_before or json.loads(os.getenv("REUSE_CLUSTER") or 'false'):
            print(
                "Skipping removal of K3d cluster - either it existed before the tests "
                "were run or REUSE_CLUSTER is set to true"
            )
            return
        self.run(
            [
                "cluster",
                "delete",
                self.name,
            ],
        )

    def executable(self):
        if shutil.which("k3d"):
            return shutil.which("k3d")
        if os.path.isfile("/tmp/k3d-linux-amd64"):
            return "/tmp/k3d-linux-amd64"
        download(
            f"https://github.com/k3d-io/k3d/releases/download/{self.KIND_VERSION}/k3d-linux-amd64"
        )
        assert not run_process(["chmod", "+x", "/tmp/k3d-linux-amd64"]).returncode
        return "/tmp/k3d-linux-amd64"

    def run(self, command):
        return run_process(
            [self.executable()] + command,
            shell=False,
            stdout=subprocess.PIPE,
            check=True,
        )

    def load_image(self, image):
        self.run(
            [
                "image",
                "import",
                "-c",
                self.name,
                image,
            ],
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
                "--set",
                "client.pullPolicy=Never",
                "--set",
                "server.pullPolicy=Never",
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


class Annotator:

    def __init__(self, mock_server, kubectl, override_name=None):
        self.mock_server = mock_server
        self.kubectl = kubectl
        self.override_name = override_name

    def do(self, key, value):
        self.kubectl.run(
            [
                "-n",
                self.mock_server.namespace,
                "annotate",
                "svc",
                self.override_name if self.override_name else self.mock_server.name,
                f"{key}={value}",
                "--overwrite"
            ]
        )


class Services:

    @classmethod
    def count(cls, kubectl, namespace):
        return len(kubectl.json(["get", "svc", "-n", namespace])["items"])
    
    @classmethod
    def names(cls, kubectl, namespace):
        return [item["metadata"]["name"] for item in kubectl.json(["get", "svc", "-n", namespace])["items"]]
