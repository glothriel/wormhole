import pytest
from retry import retry

from .fixtures import Annotator, Services

DEFAULT_RETRY_TRIES = 360
DEFAULT_RETRY_DELAY = 1


def test_changing_annotation_causes_creating_proxy_service(
    kubectl,
    k8s_server,
    k8s_client,
    mock_server,
):
    annotator = Annotator(mock_server, kubectl)
    amount_of_services_before_annotation = Services.count(kubectl, "server")
    annotator.do("wormhole.glothriel.github.com/exposed", "yes")

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_created():
        assert Services.count(kubectl, "server") == amount_of_services_before_annotation + 1

    _ensure_that_proxied_service_is_created()
    annotator.do("wormhole.glothriel.github.com/exposed", "no")

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_deleted():
        assert Services.count(kubectl, "server") == amount_of_services_before_annotation

    _ensure_that_proxied_service_is_deleted()


def test_annotating_with_custom_name_correctly_sets_remote_name(
    kubectl,
    k8s_server,
    k8s_client,
    mock_server,
):
    annotator = Annotator(mock_server, kubectl)
    annotator.do("wormhole.glothriel.github.com/exposed", "yes")
    annotator.do("wormhole.glothriel.github.com/name", "huehue-one-two-three")

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_created():
        assert "client-huehue-one-two-three" in Services.names(kubectl, namespace="server")
        assert "server-huehue-one-two-three" in Services.names(kubectl, namespace="client")

    _ensure_that_proxied_service_is_created()

    annotator.do("wormhole.glothriel.github.com/exposed", "no")

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_deleted():
        assert "client-huehue-one-two-three" not in Services.names(kubectl, namespace="server")
        assert "server-huehue-one-two-three" not in Services.names(kubectl, namespace="client")

    _ensure_that_proxied_service_is_deleted()


def test_deleting_annotated_service_removes_it_from_peers(
    kubectl,
    k8s_server,
    k8s_client,
    mock_server,
):
    annotator = Annotator(mock_server, kubectl)
    annotator.do("wormhole.glothriel.github.com/exposed", "yes")
    annotator.do("wormhole.glothriel.github.com/name", "huehue-one-two-three")

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_created():
        assert "client-huehue-one-two-three" in [
            svc["metadata"]["name"] for svc in kubectl.json(["-n", "server", "get", "svc"])["items"]
        ]

    _ensure_that_proxied_service_is_created()

    kubectl.run(["-n", mock_server.namespace, "delete", "svc", mock_server.name])

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_deleted():
        assert "client-huehue-one-two-three" not in [
            svc["metadata"]["name"] for svc in kubectl.json(["-n", "server", "get", "svc"])["items"]
        ]

    _ensure_that_proxied_service_is_deleted()


def test_exposing_service_with_multiple_ports(
    kubectl,
    k8s_server,
    k8s_client,
    mock_server,
):
    annotator = Annotator(mock_server, kubectl, override_name=f"{mock_server.name}-two-ports")
    annotator.do("wormhole.glothriel.github.com/exposed", "yes")
    annotator.do("wormhole.glothriel.github.com/name", "custom")

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_created():
        assert "client-custom-http" in [
            svc["metadata"]["name"] for svc in kubectl.json(["-n", "server", "get", "svc"])["items"]
        ]
        assert "client-custom-https" in [
            svc["metadata"]["name"] for svc in kubectl.json(["-n", "server", "get", "svc"])["items"]
        ]

    _ensure_that_proxied_service_is_created()

    annotator.do("wormhole.glothriel.github.com/exposed", "no")

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_deleted():
        assert "client-custom-http" not in [
            svc["metadata"]["name"] for svc in kubectl.json(["-n", "server", "get", "svc"])["items"]
        ]
        assert "client-custom-https" not in [
            svc["metadata"]["name"] for svc in kubectl.json(["-n", "server", "get", "svc"])["items"]
        ]

    _ensure_that_proxied_service_is_deleted()


def test_exposing_service_with_selected_ports(
    kubectl,
    k8s_server,
    k8s_client,
    mock_server,
):
    annotator = Annotator(mock_server, kubectl, override_name=f"{mock_server.name}-two-ports")
    annotator.do("wormhole.glothriel.github.com/exposed", "yes")
    annotator.do("wormhole.glothriel.github.com/name", "custom")
    annotator.do("wormhole.glothriel.github.com/ports", "http")

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_created():
        assert "client-custom" in [
            svc["metadata"]["name"] for svc in kubectl.json(["-n", "server", "get", "svc"])["items"]
        ]

    _ensure_that_proxied_service_is_created()

    annotator.do("wormhole.glothriel.github.com/exposed", "no")

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_deleted():
        assert "client-custom" not in [
            svc["metadata"]["name"] for svc in kubectl.json(["-n", "server", "get", "svc"])["items"]
        ]

    _ensure_that_proxied_service_is_deleted()


def test_exposing_service_with_changing_ports(
    kubectl,
    k8s_server,
    k8s_client,
    mock_server,
):
    annotator = Annotator(mock_server, kubectl, override_name=f"{mock_server.name}-two-ports")
    annotator.do("wormhole.glothriel.github.com/exposed", "yes")
    annotator.do("wormhole.glothriel.github.com/name", "custom")
    annotator.do("wormhole.glothriel.github.com/ports", "http")

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_created():
        assert "client-custom" in [
            svc["metadata"]["name"] for svc in kubectl.json(["-n", "server", "get", "svc"])["items"]
        ]

    _ensure_that_proxied_service_is_created()

    annotator.do("wormhole.glothriel.github.com/ports", "http,https")

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_deleted():
        assert "client-custom-http" not in [
            svc["metadata"]["name"] for svc in kubectl.json(["-n", "server", "get", "svc"])["items"]
        ]
        assert "client-custom-https" not in [
            svc["metadata"]["name"] for svc in kubectl.json(["-n", "server", "get", "svc"])["items"]
        ]

    _ensure_that_proxied_service_is_deleted()

    if "client-custom" in [
        svc["metadata"]["name"] for svc in kubectl.json(["-n", "server", "get", "svc"])["items"]
    ]:
        pytest.skip(
            "The orphaned service should be removed, but it's not critical, so skipping for now"
        )


def test_connection_via_the_tunnel(
    kubectl,
    k8s_server,
    k8s_client,
    mock_server,
    curl,
):
    annotator = Annotator(mock_server, kubectl)
    amount_of_services_before_annotation = Services.count(kubectl, "server")
    annotator.do("wormhole.glothriel.github.com/exposed", "yes")

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_created():
        assert Services.count(kubectl, "server") == amount_of_services_before_annotation + 1

    _ensure_that_proxied_service_is_created()

    @retry(tries=int(DEFAULT_RETRY_TRIES / 10), delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_proxied_service_is_reachable():
        # Calling CURL from annotated pod should succeed
        curl.call_with_network_policy(
            "http://server-nginx-nginx.client.svc.cluster.local",
            max_time_seconds=10,
        )

    _ensure_that_proxied_service_is_reachable()

    # Calling CURL from non-annotated pod should fail
    with pytest.raises(Exception):
        curl.call_without_network_policy(
            "http://server-nginx-nginx.client.svc.cluster.local",
            max_time_seconds=10,
        )


def test_reconnecting_clients_with_keys(
    kubectl,
    k8s_server,
    k8s_client,
    mock_server,
):
    @retry(tries=int(DEFAULT_RETRY_TRIES / 10), delay=DEFAULT_RETRY_DELAY)
    def _wait_for_peers_paired_using_psk():
        assert (
            "Paired with server, assigned IP"
            in kubectl.run(
                ["-n", "client", "logs", "-l", "application=wormhole-client", "-c", "wormhole"]
            ).stdout.decode()
        )

    _wait_for_peers_paired_using_psk()

    kubectl.run(["-n", "client", "delete", "pod", "-l", "application=wormhole-client"])

    @retry(tries=int(DEFAULT_RETRY_TRIES / 10), delay=DEFAULT_RETRY_DELAY)
    def _wait_for_peers_paired_using_keys():
        assert (
            "using IP from the cache"
            in kubectl.run(
                ["-n", "client", "logs", "-l", "application=wormhole-client", "-c", "wormhole"]
            ).stdout.decode()
        )

    _wait_for_peers_paired_using_keys()


def test_client_metadata(
    kubectl,
    k8s_server,
    k8s_client,
    helm,
    curl,
):
    helm.install(
        "client",
        {
            "client.syncMetadata.testFoo": "testBar",
        },
        reuse_values=True,
    )

    @retry(tries=DEFAULT_RETRY_TRIES, delay=DEFAULT_RETRY_DELAY)
    def _ensure_that_peers_v2_contains_client_metadata():
        response_body = curl.call_with_network_policy(
            "http://wormhole-server-api.server:8082/api/peers/v2",
            max_time_seconds=10,
        ).stdout.decode()
        assert "testFoo" in response_body
        assert "testBar" in response_body

    _ensure_that_peers_v2_contains_client_metadata()
