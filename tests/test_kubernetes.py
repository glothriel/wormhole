import pytest
from retry import retry


def test_changing_annotation_causes_creating_proxy_service(
    kubectl,
    helm,
    fresh_cluster,
    wormhole_image,
    wireguard_image,
    nginx_image,
    docker_images_loaded_into_cluster,
    mock_server,
):
    kubectl.run(["create", "namespace", "server"])
    helm.install(
        "server",
        {
            "server.enabled": True,
            "server.wg.publicHost": "wormhole-server-server.server.svc.cluster.local",
            "docker.image": wormhole_image.split(":")[0],
            "docker.version": wormhole_image.split(":")[1],
            "docker.wgImage": wireguard_image.split(":")[0],
            "docker.wgVersion": wireguard_image.split(":")[1],
            "docker.nginxImage": nginx_image.split(":")[0],
            "docker.nginxVersion": nginx_image.split(":")[1],
            "docker.registry": "",
        },
    )

    kubectl.run(["create", "namespace", "client"])
    helm.install(
        "client",
        {
            "client.enabled": True,
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

    amount_of_services_before_annotation = len(
        kubectl.json(["-n", "server", "get", "svc"])["items"]
    )

    # Set annotation to yes - enable proxying
    kubectl.run(
        [
            "-n",
            mock_server.namespace,
            "annotate",
            "svc",
            mock_server.name,
            "wormhole.glothriel.github.com/exposed=yes",
        ]
    )

    @retry(tries=60, delay=1)
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
            mock_server.namespace,
            "annotate",
            "--overwrite",
            "svc",
            mock_server.name,
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


# def test_client_disconnect_causes_deletion_of_related_proxy_services(
#     kubectl, helm, fresh_cluster, docker_image_loaded_into_cluster
# ):
#     kubectl.run(["create", "namespace", "server"])
#     helm.install("server", {"server.enabled": True, "server.acceptor": "dummy"})

#     kubectl.run(["create", "namespace", "client"])
#     helm.install(
#         "client",
#         {
#             "client.enabled": True,
#             "client.name": "testclient",
#             "client.serverDsn": "ws://wormhole-server-server.server:8080/wh/tunnel",
#         },
#     )

#     kubectl.run(["create", "namespace", "mocks"])
#     kubectl.run(["apply", "-f", "kubernetes/raw/mocks"])

#     amount_of_services_before_annotation = len(
#         kubectl.json(["-n", "server", "get", "svc"])["items"]
#     )

#     # Set annotation to yes - enable proxying
#     kubectl.run(
#         [
#             "-n",
#             "mocks",
#             "annotate",
#             "svc",
#             "wormhole-mocks",
#             "wormhole.glothriel.github.com/exposed=yes",
#         ]
#     )

#     @retry(tries=10, delay=1)
#     def _ensure_that_proxied_service_is_created():
#         assert (
#             len(kubectl.json(["-n", "server", "get", "svc"])["items"])
#             == amount_of_services_before_annotation + 1
#         )

#     _ensure_that_proxied_service_is_created()

#     kubectl.run(["delete", "namespace", "client"])

#     @retry(tries=60, delay=1)
#     def _ensure_that_proxied_service_is_deleted():
#         assert (
#             len(kubectl.json(["-n", "server", "get", "svc"])["items"])
#             == amount_of_services_before_annotation
#         )

#     _ensure_that_proxied_service_is_deleted()
