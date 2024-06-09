import logging
import os
import subprocess
import tempfile
import sys

import pytest

from .fixtures import Helm, K3dCluster, Kubectl, MockServer, Curl

logger = logging.getLogger(__name__)

TEST_SERVER_PORT = 1234


def run_process(process, **kwargs):
    print("\n>>> " + " ".join(process))
    rt = subprocess.run(
        process,
        **kwargs,
    )
    try:
        rt.check_returncode()
    finally:
        if rt.stdout:
            logger.info(rt.stdout.decode())
            logger.info(f"Return code: {rt.returncode}")
        if rt.stderr:
            stderr = rt.stderr.decode()
            if stderr:
                logger.error(stderr)
    return rt


@pytest.fixture()
def mock_server(fresh_cluster, wormhole_image, kubectl):
    server = MockServer(kubectl, wormhole_image)
    try:
        yield server.start()
    finally:
        server.stop()


@pytest.fixture(scope="session")
def kubernetes_cluster():
    cluster = K3dCluster("pytest")
    try:
        cluster.create()
        yield cluster
    finally:
        cluster.delete()


@pytest.fixture(scope="session")
def kubectl(kubernetes_cluster):
    yield Kubectl(kubernetes_cluster)


@pytest.fixture()
def fresh_cluster(
    kubernetes_cluster,
    docker_images_loaded_into_cluster
):
    kubectl = Kubectl(kubernetes_cluster)
    starting_namespaces = set(
        [
            'kube-system',
            'default',
            'local-path-storage',
            'kube-node-lease',
            'kube-public',
        ]
    )
    try:
        yield kubernetes_cluster
    finally:
        finishing_namespaces = set(
            [item["metadata"]["name"] for item in kubectl.json(["get", "namespaces"])["items"]]
        )
        for namespace_to_be_deleted in finishing_namespaces - starting_namespaces:
            print(f"Deleting namespace {namespace_to_be_deleted}")
            kubectl.run(["delete", "namespace", namespace_to_be_deleted])
            print(f"Deleted namespace {namespace_to_be_deleted}")


@pytest.fixture(scope='session')
def helm(kubernetes_cluster):
    yield Helm(kubernetes_cluster)


@pytest.fixture()
def server_installed_with_helm(kubectl, helm, fresh_cluster):
    kubectl.run(["create", "ns", "wormhole-server"])
    helm.run(["install", "-n", "wormhole-server", "server", "kubernetes/helm"])
    yield


@pytest.fixture(scope="session")
def wormhole_image():
    # Define the Docker image and build parameters
    image_name = "wormhole-controller:latest"
    context_path = os.path.abspath(".")
    dockerfile_path = "./docker/goDockerfile"
    build_args = {
        "USER_ID": subprocess.check_output(["id", "-u"]).decode().strip(),
        "GROUP_ID": subprocess.check_output(["id", "-g"]).decode().strip(),
        "VERSION": "dev",
        "PROJECT": "..",
    }

    # Build the Docker image
    build_command = ["docker", "build", "-t", image_name, "-f", dockerfile_path, context_path] + [
        j
        for sub in [("--build-arg", f"{key}={value}") for key, value in build_args.items()]
        for j in sub
    ]

    run_process(build_command, shell=False, stdout=sys.stdout, check=True)

    # Yield the image name for use in tests
    yield image_name


@pytest.fixture(scope="session")
def wireguard_image():
    # Define the Docker image and build parameters
    image_name = "wormhole-wireguard:latest"
    context_path = os.path.abspath("docker")
    dockerfile_path = "./docker/wgDockerfile"

    # Build the Docker image
    build_command = ["docker", "build", "-t", image_name, "-f", dockerfile_path, context_path]

    run_process(build_command, shell=False, stdout=sys.stdout, check=True)

    # Yield the image name for use in tests
    yield image_name


@pytest.fixture(scope="session")
def nginx_image():
    # Define the Docker image and build parameters
    image_name = "wormhole-nginx:latest"
    context_path = os.path.abspath("docker")
    dockerfile_path = "./docker/nginxDockerfile"

    # Build the Docker image
    build_command = ["docker", "build", "-t", image_name, "-f", dockerfile_path, context_path]

    run_process(build_command, shell=False, stdout=sys.stdout, check=True)

    # Yield the image name for use in tests
    yield image_name


@pytest.fixture(scope="session")
def docker_images_loaded_into_cluster(kubernetes_cluster, wormhole_image, wireguard_image, nginx_image):
    kubernetes_cluster.load_image(wormhole_image)
    kubernetes_cluster.load_image(wireguard_image)
    kubernetes_cluster.load_image(nginx_image)
    yield {
        'wormhole': wormhole_image,
        'wireguard': wireguard_image,
        'nginx': nginx_image,
    }


@pytest.fixture(scope="session")
def curl(kubectl):
    c = Curl(kubectl)
    c.start()
    try:
        yield c
    finally:
        c.stop()


@pytest.fixture()
def k8s_server(
    kubectl,
    helm,
    wormhole_image,
    wireguard_image,
    nginx_image,
    fresh_cluster,
):
    kubectl.run(["create", "namespace", "server"])
    helm.install(
        "server",
        {
            "server.enabled": True,
            "networkPolicies.enabled": True,
            "server.wg.publicHost": "wormhole-server-server.server.svc.cluster.local",
            "server.service.type": "ClusterIP",
            "docker.image": wormhole_image.split(":")[0],
            "docker.version": wormhole_image.split(":")[1],
            "docker.wgImage": wireguard_image.split(":")[0],
            "docker.wgVersion": wireguard_image.split(":")[1],
            "docker.nginxImage": nginx_image.split(":")[0],
            "docker.nginxVersion": nginx_image.split(":")[1],
            "docker.registry": "",
        },
    )


@pytest.fixture()
def k8s_client(
    kubectl,
    helm,
    wormhole_image,
    wireguard_image,
    nginx_image,
    fresh_cluster,
):

    kubectl.run(["create", "namespace", "client"])
    helm.install(
        "client",
        {
            "client.enabled": True,
            "networkPolicies.enabled": True,
            "client.name": "client",
            "client.serverDsn": "http://wormhole-server-server.server.svc.cluster.local:8080",
            "docker.image": wormhole_image.split(":")[0],
            "docker.version": wormhole_image.split(":")[1],
            "docker.wgImage": wireguard_image.split(":")[0],
            "docker.wgVersion": wireguard_image.split(":")[1],
            "docker.nginxImage": nginx_image.split(":")[0],
            "docker.nginxVersion": nginx_image.split(":")[1],
            "docker.registry": "",
        },
    )
