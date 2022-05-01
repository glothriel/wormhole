import pytest
import requests
from retry import retry

from .fixtures import launched_in_background, Client, get_number_of_running_goroutines


def test_server_goroutines_do_not_leak_when_connecting_and_disconnecting_clients(
    executable, server, mock_server
):
    starting_goroutines = get_number_of_running_goroutines(server.metrics_port)

    for i in range(10):
        with launched_in_background(Client(executable, exposes=[f"localhost:{mock_server.port}"])):    
            @retry(delay=0.05, tries=20)
            def _ensure_mock_app_status(exposed=True):
                assert len(
                    requests.get(f"http://localhost:{server.admin_port}/v1/apps").json()
                ) == (1 if exposed else 0)
            _ensure_mock_app_status(exposed=True)
            # List the apps
            apps = requests.get(f"http://localhost:{server.admin_port}/v1/apps").json()
            # Call the proxied app
            requests.get(f'http://{apps[0]["endpoint"]}', timeout=1)
        _ensure_mock_app_status(exposed=False)
    
    ending_goroutines = get_number_of_running_goroutines(server.metrics_port)

    # One extra goroutine allowed
    assert ending_goroutines <= starting_goroutines + 1


def test_server_goroutines_do_not_leak_when_passing_messages(
    executable, server, mock_server
):
    with launched_in_background(Client(executable, exposes=[f"localhost:{mock_server.port}"])):    
        @retry(delay=0.05, tries=20)
        def _ensure_mock_app_status(exposed=True):
            assert len(
                requests.get(f"http://localhost:{server.admin_port}/v1/apps").json()
            ) == (1 if exposed else 0)
        _ensure_mock_app_status(exposed=True)
        # List the apps
        apps = requests.get(f"http://localhost:{server.admin_port}/v1/apps").json()
        
        starting_goroutines = get_number_of_running_goroutines(server.metrics_port)
        # Call the proxied app
        for i in range(50):
            requests.get(f'http://{apps[0]["endpoint"]}', timeout=1)

        ending_goroutines = get_number_of_running_goroutines(server.metrics_port)
    
        # One extra goroutine allowed
        assert ending_goroutines <= starting_goroutines + 1


@pytest.mark.skip(reason="Fails - we have a leak :(")
def test_client_goroutines_do_not_leak_when_passing_messages(
    executable, server, mock_server
):
    with launched_in_background(Client(executable, exposes=[f"localhost:{mock_server.port}"])) as client:    
        @retry(delay=0.05, tries=20)
        def _ensure_mock_app_status(exposed=True):
            assert len(
                requests.get(f"http://localhost:{server.admin_port}/v1/apps").json()
            ) == (1 if exposed else 0)
        _ensure_mock_app_status(exposed=True)
        
        # List the apps
        apps = requests.get(f"http://localhost:{server.admin_port}/v1/apps").json()
        
        starting_goroutines = get_number_of_running_goroutines(client.metrics_port)

        # Call the proxied app
        for i in range(50):
            requests.get(f'http://{apps[0]["endpoint"]}', timeout=1)

        ending_goroutines = get_number_of_running_goroutines(client.metrics_port)
    
        # One extra goroutine allowed
        assert ending_goroutines <= starting_goroutines + 1, "It appears, that sending messages causes goroutine " \
            "leaks on the client :("
