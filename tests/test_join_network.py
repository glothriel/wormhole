import os
from retry import retry


def assert_wireguard_config_params(config_path, address, allowed_ips):
    with open(config_path) as f:
        lines = f.readlines()
    assert f"Address = {address}\n" in lines
    assert f"AllowedIPs = {allowed_ips}\n" in lines


def test_wireguard_configs_created(
        executable, server, client
):
    @retry(tries=30, delay=1)
    def _ensure_wireguard_configs_were_created():
        assert os.path.exists(server.wireguard_config_path)
        assert os.path.exists(client.wireguard_config_path)

    _ensure_wireguard_configs_were_created()

    parts = server.wireguard_address.split(".")
    parts[-1] = int(parts[-1]) + 1
    first_client_ip = ".".join(map(str, parts))
    assert_wireguard_config_params(
        server.wireguard_config_path,
        f"{server.wireguard_address}/{server.wireguard_subnet}",
        f"{first_client_ip}/32,{server.wireguard_address}/32",
    )
