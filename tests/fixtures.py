import json
import os
import shutil
import subprocess
from contextlib import contextmanager

import psutil
import requests
from retry import retry


def run_process(command, *args, **kwargs):
    print(f">>> {' '.join(command)}")
    return subprocess.run(command, *args, **kwargs)


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


class K3dCluster:

    K3D_VERSION = "v5.6.3"

    def __init__(self, name):
        self.name = name
        self.existed_before = False
        self.kubeconfig = os.path.join("/tmp", f"k3d-{self.name}-kubeconfig")

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
            f"https://github.com/k3d-io/k3d/releases/download/{self.K3D_VERSION}/k3d-linux-amd64",
            "/tmp/k3d-linux-amd64"
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
