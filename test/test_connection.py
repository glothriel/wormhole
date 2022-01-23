import requests

from .fixtures import launched_in_background, Client, MockServer


def test_hello_world_is_returned_via_tunnel(mock_server, client, server):
    apps = requests.get(f"http://localhost:{server.admin_port}/v1/apps").json()
    assert len(apps) == 1, "One app should be registered"
    assert (
        requests.get(f'http://localhost:{apps[0]["port"]}', timeout=2).text
        == "Hello world!"
    )


def test_two_distinct_clients_can_be_connected_and_are_properly_visible_in_the_api(
    executable, server, mock_server
):
    with launched_in_background(MockServer(executable, response="Bla!", port=4321)) as second_mock_server:
        with launched_in_background(Client(executable, exposes=[f"localhost:{mock_server.port}"])):
            with launched_in_background(Client(
                executable,
                exposes=[("app-from-client-two", f"localhost:{second_mock_server.port}")],
            )):

                api_response = requests.get(
                    f"http://localhost:{server.admin_port}/v1/apps"
                ).json()

                assert list(sorted([item["app"] for item in api_response])) == [
                    "app-from-client-two",
                    "localhost:1234",
                ], "Exactly two clients should be connected, each with one distinct app"

                assert [
                    requests.get(f'http://localhost:{app["port"]}', timeout=2).text for app in api_response
                ] == [
                    "Hello world!",
                    "Bla!"
                ], (
                    "There are two distinct apps (mock servers), each of them having a separate client connected, "
                    "so there should be two disting responses"
                )


def test_peer_disappears_from_api_when_client_disconnects(
    executable, server, mock_server
):
    apps_url = f"http://localhost:{server.admin_port}/v1/apps"

    # When no client connected
    assert len(requests.get(apps_url).json()) == 0

    try:
        peer = Client(executable, exposes=[f"localhost:{mock_server.port}"]).start()

        # One client connected
        assert len(requests.get(apps_url).json()) == 1
    finally:
        peer.stop()

    # After client disconnect
    assert len(requests.get(apps_url).json()) == 0
