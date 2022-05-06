import pytest
import requests
from retry import retry

from .fixtures import (
    Client,
    get_number_of_opened_files,
    get_number_of_running_goroutines,
    launched_in_background,
)


class LeakTestOptions:
    def __init__(self, scenario, counter_func, allow_extra_resources=2):
        self.scenario = scenario
        self.counter_func = counter_func
        self.allow_extra_resources = allow_extra_resources

    def __str__(self):
        return self.scenario


@pytest.mark.parametrize(
    "opts",
    (
        LeakTestOptions(
            "Server, opened files", lambda server: get_number_of_opened_files(server)
        ),
        LeakTestOptions(
            "Server, running goroutines",
            lambda server: get_number_of_running_goroutines(server.metrics_port),
        ),
    ),
)
def test_resource_leaks_when_connecting_and_disconnecting_clients(
    executable, server, mock_server, opts
):
    starting_resources = opts.counter_func(server)

    for _ in range(10):
        with launched_in_background(
            Client(executable, exposes=[f"localhost:{mock_server.port}"])
        ):

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

    ending_resources = opts.counter_func(server)

    assert ending_resources <= (
        starting_resources + opts.allow_extra_resources
    ), f"It appears, that we have a leak on {opts.scenario} :("


@pytest.mark.parametrize(
    "opts",
    (
        LeakTestOptions(
            "Client, opened files",
            lambda client, server: get_number_of_opened_files(client),
        ),
        LeakTestOptions(
            "Client, running goroutines",
            lambda client, server: get_number_of_running_goroutines(
                client.metrics_port
            ),
        ),
        LeakTestOptions(
            "Server, opened files",
            lambda client, server: get_number_of_opened_files(server),
        ),
        LeakTestOptions(
            "Server, running goroutines",
            lambda client, server: get_number_of_running_goroutines(
                server.metrics_port
            ),
        ),
    ),
)
def test_resource_leaks_when_passing_messages(executable, server, mock_server, opts):
    with launched_in_background(
        Client(executable, exposes=[f"localhost:{mock_server.port}"])
    ) as client:

        @retry(delay=0.05, tries=20)
        def _ensure_mock_app_exposed():
            assert requests.get(f"http://localhost:{server.admin_port}/v1/apps").json()

        _ensure_mock_app_exposed()

        # List the apps
        apps = requests.get(f"http://localhost:{server.admin_port}/v1/apps").json()

        starting_resources = opts.counter_func(client, server)

        # Call the proxied app
        for i in range(50):
            print(i)
            requests.get(f'http://{apps[0]["endpoint"]}', timeout=1)

        # Give it a second to get rid of the resources (close files, goroutines, etc)
        @retry(delay=0.1, tries=20)
        def _ensure_resource_not_leaking():
            ending_resources = opts.counter_func(client, server)
            assert (
                ending_resources <= starting_resources + opts.allow_extra_resources
            ), f"It appears, that we have a leak on {opts.scenario} :("

        _ensure_resource_not_leaking()
