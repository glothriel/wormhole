import logging
import os
import subprocess
import tempfile

import pytest

from .fixtures import Client, Helm, KindCluster, Kubectl, MockServer, MySQLServer, Server

logger = logging.getLogger(__name__)

TEST_SERVER_PORT = 1234


def run_process(process, **kwargs):
    logger.info(" ".join(process))
    rt = subprocess.run(process, stdout=subprocess.PIPE, stderr=subprocess.PIPE, **kwargs)
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
        server.start()
        yield server
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


@pytest.fixture(scope="session")
def kind_cluster():
    cluster = KindCluster("pytest")
    try:
        cluster.create()
        yield cluster
    finally:
        cluster.delete()


@pytest.fixture(scope="session")
def kubectl(kind_cluster):
    yield Kubectl(kind_cluster)


@pytest.fixture()
def fresh_cluster(kind_cluster):
    kubectl = Kubectl(kind_cluster)
    starting_namespaces = set(
        [item["metadata"]["name"] for item in kubectl.json(["get", "namespaces"])["items"]]
    )
    try:
        yield kind_cluster
    finally:
        finishing_namespaces = set(
            [item["metadata"]["name"] for item in kubectl.json(["get", "namespaces"])["items"]]
        )
        for namespace_to_be_deleted in finishing_namespaces - starting_namespaces:
            print(f"Deleting namespace {namespace_to_be_deleted}")
            kubectl.run(["delete", "namespace", namespace_to_be_deleted])
            print(f"Deleted namespace {namespace_to_be_deleted}")


@pytest.fixture()
def helm(kind_cluster):
    yield Helm(kind_cluster)


@pytest.fixture()
def server_installed_with_helm(kubectl, helm, fresh_cluster):
    kubectl.run(["create", "ns", "wormhole-server"])
    helm.run(["install", "-n", "wormhole-server", "server", "kubernetes/helm"])
    yield


@pytest.fixture(scope="session")
def docker_image():
    image_name = "ghcr.io/glothriel/wormhole:pytest"
    subprocess.run(
        ["docker", "build", "-t", image_name, "."],
        shell=False,
        stdout=subprocess.PIPE,
        check=True,
        cwd=os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
    )
    yield image_name


@pytest.fixture(scope="session")
def docker_image_loaded_into_cluster(kind_cluster, docker_image):
    kind_cluster.load_image(docker_image)
    yield docker_image
