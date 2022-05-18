import requests
from retry import retry

from .fixtures import Client, Server, launched_in_background


def test_server_acceptor_can_successfully_accept_a_fingerprint(executable, mock_server):
    with launched_in_background(Server(executable, acceptor="server")) as server:
        with launched_in_background(Client(executable, [mock_server.endpoint()])):
            assert len(requests.get(server.admin("/v1/requests")).json()) == 1
            # Read first fingerprint in accept queue
            fingerprint = requests.get(server.admin("/v1/requests")).json()[0]
            # Accept the fingerprint
            requests.post(server.admin(f"/v1/requests/{fingerprint}"))

            @retry(tries=3, delay=0.1)
            def _ensure_app_is_proxied():
                assert (
                    requests.get(
                        f"http://{requests.get(server.admin('/v1/apps')).json()[0]['endpoint']}"
                    ).status_code
                    == 200
                )

            # Wait until peers introduce each other and app starts to be proxied
            _ensure_app_is_proxied()


def test_client_is_disconnected_and_terminated_when_fingerprint_is_discarded(
    executable, mock_server
):
    with launched_in_background(Server(executable, acceptor="server")) as server:
        with launched_in_background(
            Client(executable, [mock_server.endpoint()])
        ) as client:
            assert len(requests.get(server.admin("/v1/requests")).json()) == 1
            fingerprint = requests.get(server.admin("/v1/requests")).json()[0]
            requests.delete(server.admin(f"/v1/requests/{fingerprint}"))

            @retry(tries=3, delay=0.1)
            def _ensure_client_is_terminated():
                client.process.poll() is None

            _ensure_client_is_terminated()


def test_first_client_is_disconnected_when_second_with_the_same_key_attempts_to_connect(
    executable, mock_server
):
    with launched_in_background(Server(executable, acceptor="server")):
        with launched_in_background(
            Client(executable, [mock_server.endpoint()])
        ) as first_client:
            with launched_in_background(Client(executable, [mock_server.endpoint()])):

                @retry(tries=3, delay=0.1)
                def _ensure_client_is_terminated():
                    first_client.process.poll() is None

                _ensure_client_is_terminated()
