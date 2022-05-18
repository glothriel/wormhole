import pytest
from retry import retry


@pytest.mark.parametrize("pvc", (True, False))
def test_helm_chart_is_installable(
    kubectl, helm, fresh_cluster, docker_image_loaded_into_cluster, pvc
):
    kubectl.run(["create", "namespace", "server"])
    helm.install(
        "server",
        {"server.enabled": True, "server.acceptor": "dummy", "server.pvc.enabled": pvc},
    )

    kubectl.run(["create", "namespace", "client"])
    helm.install(
        "client",
        {
            "client.enabled": True,
            "client.pvc.enabled": pvc,
            "client.name": "testclient",
            "client.serverDsn": "ws://wormhole-server-server.server:8080",
        },
    )


def test_changing_annotation_causes_creating_and_deleting_proxy_service(
    kubectl, helm, fresh_cluster, docker_image_loaded_into_cluster
):
    kubectl.run(["create", "namespace", "server"])
    helm.install(
        "server",
        {
            "server.enabled": True,
            "server.acceptor": "dummy",
            "server.pvc.enabled": True,
        },
    )

    kubectl.run(["create", "namespace", "client"])
    helm.install(
        "client",
        {
            "client.enabled": True,
            "client.pvc.enabled": True,
            "client.name": "testclient",
            "client.serverDsn": "ws://wormhole-server-server.server:8080",
        },
    )

    kubectl.run(["create", "namespace", "mocks"])
    kubectl.run(["apply", "-f", "kubernetes/raw/mocks"])

    amount_of_services_before_annotation = len(
        kubectl.json(["-n", "server", "get", "svc"])["items"]
    )

    # Set annotation to yes - enable proxying
    kubectl.run(
        [
            "-n",
            "mocks",
            "annotate",
            "svc",
            "wormhole-mocks",
            "wormhole.glothriel.github.com/exposed=yes",
        ]
    )

    @retry(tries=10, delay=1)
    def _ensure_that_proxied_service_is_created():
        assert (
            len(kubectl.json(["-n", "server", "get", "svc"])["items"])
            == amount_of_services_before_annotation + 1
        )

    _ensure_that_proxied_service_is_created()

    # Set annotation to no - disable proxying
    kubectl.run(
        [
            "-n",
            "mocks",
            "annotate",
            "--overwrite",
            "svc",
            "wormhole-mocks",
            "wormhole.glothriel.github.com/exposed=no",
        ]
    )

    @retry(tries=60, delay=1)
    def _ensure_that_proxied_service_is_deleted():
        assert (
            len(kubectl.json(["-n", "server", "get", "svc"])["items"])
            == amount_of_services_before_annotation
        )

    _ensure_that_proxied_service_is_deleted()


def test_client_disconnect_causes_deletion_of_related_proxy_services(
    kubectl, helm, fresh_cluster, docker_image_loaded_into_cluster
):
    kubectl.run(["create", "namespace", "server"])
    helm.install("server", {"server.enabled": True, "server.acceptor": "dummy"})

    kubectl.run(["create", "namespace", "client"])
    helm.install(
        "client",
        {
            "client.enabled": True,
            "client.name": "testclient",
            "client.serverDsn": "ws://wormhole-server-server.server:8080",
        },
    )

    kubectl.run(["create", "namespace", "mocks"])
    kubectl.run(["apply", "-f", "kubernetes/raw/mocks"])

    amount_of_services_before_annotation = len(
        kubectl.json(["-n", "server", "get", "svc"])["items"]
    )

    # Set annotation to yes - enable proxying
    kubectl.run(
        [
            "-n",
            "mocks",
            "annotate",
            "svc",
            "wormhole-mocks",
            "wormhole.glothriel.github.com/exposed=yes",
        ]
    )

    @retry(tries=10, delay=1)
    def _ensure_that_proxied_service_is_created():
        assert (
            len(kubectl.json(["-n", "server", "get", "svc"])["items"])
            == amount_of_services_before_annotation + 1
        )

    _ensure_that_proxied_service_is_created()

    kubectl.run(["delete", "namespace", "client"])

    @retry(tries=60, delay=1)
    def _ensure_that_proxied_service_is_deleted():
        assert (
            len(kubectl.json(["-n", "server", "get", "svc"])["items"])
            == amount_of_services_before_annotation
        )

    _ensure_that_proxied_service_is_deleted()
