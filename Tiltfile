# allow_k8s_contexts(k8s_context())

default_registry(
    'localhost:33255',
    host_from_cluster='wormhole:5000'
)

# Define the Docker image build
docker_build(
    'wormhole', 
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
        sync('./main.go', '/src/main.go'),
        sync('./pkg', '/src/pkg')
    ]
)

# Define the Docker image build
docker_build(
    'wireguard', 
    context='docker',
    dockerfile='./docker/wgDockerfile',
)

servers = ["server"]
clients = ["dev1", "dev2"]

[k8s_yaml(blob("""
apiVersion: v1
kind: Namespace
metadata:
  name: {ns}
""".replace("{ns}", ns))) for ns in (servers + clients)]

for server in servers:
    k8s_yaml(helm("./kubernetes/helm", namespace=server, set=[
        "server.enabled=true",
        "server.acceptor=dummy",
        "server.resources.limits.memory=2Gi",
        "server.securityContext.runAsUser=0",
        "server.securityContext.runAsGroup=0",
        "server.securityContext.runAsNonRoot=false",
        "server.containerSecurityContext.readOnlyRootFilesystem=false",
        "server.containerSecurityContext.privileged=true",
        "server.containerSecurityContext.allowPrivilegeEscalation=true",
        "server.wg.publicHost=wormhole-server-chart.server.svc.cluster.local",
        "docker.image=wormhole",
        "docker.wgImage=wireguard",
        "docker.registry=",
        "devMode.enabled=true",
    ]))

for client in clients:
    k8s_yaml(helm("./kubernetes/helm", namespace=client, name=client, set=[
        "client.enabled=true",
        "client.name=" + client,
        "client.serverDsn=http://wormhole-server-chart-peering.server.svc.cluster.local:8080",
        "client.resources.limits.memory=2Gi",
        "client.securityContext.runAsUser=0",
        "client.securityContext.runAsGroup=0",
        "client.securityContext.runAsNonRoot=false",
        "client.containerSecurityContext.readOnlyRootFilesystem=false",
        "client.containerSecurityContext.privileged=true",
        "client.containerSecurityContext.allowPrivilegeEscalation=true",
        "docker.image=wormhole",
        "docker.wgImage=wireguard",
        "docker.registry=",
        "devMode.enabled=true",
    ]))

