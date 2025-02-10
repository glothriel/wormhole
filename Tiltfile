# allow_k8s_contexts(k8s_context())

default_registry(
    'localhost:33255',
    host_from_cluster='wormhole:5000'
)

docker_build(
    'wormhole-controller', 
    context='.', 
    dockerfile='./docker/goDockerfile',
    target='dev',
    build_args={
        'USER_ID': str(local('id -u')),
        'GROUP_ID': str(local('id -g')),
        'VERSION': 'dev',
        'PROJECT': '..'
    },
    live_update=[
        sync('./main.go', '/src-tmp/main.go'),
        sync('./pkg', '/src-tmp/pkg'),
        sync('./go.sum', '/src-tmp/go.sum'),
        sync('./go.mod', '/src-tmp/go.mod')
    ]
)

docker_build(
    'wormhole-wireguard', 
    context='docker',
    dockerfile='./docker/wgDockerfile',
)

docker_build(
    'wormhole-nginx', 
    context='docker',
    dockerfile='./docker/nginxDockerfile',
)

servers = ["server"]
clients = ["dev1", "dev2"]

[k8s_yaml(blob("""
apiVersion: v1
kind: Namespace
metadata:
  name: {ns}
""".replace("{ns}", ns))) for ns in (servers + clients)]

k8s_yaml('./kubernetes/raw/mocks/all.yaml')

for server in servers:
    k8s_yaml(helm("./kubernetes/helm", namespace=server, set=[
        "server.enabled=true",
        "server.resources.limits.memory=2Gi",
        "server.wg.publicHost=wormhole-server.server.svc.cluster.local",
        "server.service.type=ClusterIP",
        "docker.image=wormhole-controller",
        "docker.wgImage=wormhole-wireguard",
        "docker.nginxImage=wormhole-nginx",
        "networkPolicies.enabled=true",
        "docker.registry=",
        "devMode.enabled=true",
    ]))

for client in clients:
    k8s_yaml(helm("./kubernetes/helm", namespace=client, name=client, set=[
        "client.enabled=true",
        "client.name=" + client,
        "client.serverDsn=http://wormhole-server.server.svc.cluster.local:8080",
        "client.resources.limits.memory=2Gi",
        "docker.image=wormhole-controller",
        "docker.wgImage=wormhole-wireguard",
        "docker.nginxImage=wormhole-nginx",
        "networkPolicies.enabled=true",
        "docker.registry=",
        "devMode.enabled=true",
    ]))

