# allow_k8s_contexts(k8s_context())

default_registry(
    'localhost:33255',
    host_from_cluster='wormhole:5000'
)

# Define the Docker image build
docker_build(
    'wormhole', 
    context='.', 
    dockerfile='./docker/Dockerfile.go',
    target='dev',
    build_args={
        'USER_ID': str(local('id -u')),
        'GROUP_ID': str(local('id -g')),
        'VERSION': 'dev',
        # You might need to adjust 'PROJECT' or remove it depending on your Dockerfile and context
        'PROJECT': '..'
    },
    # Specify the live update configuration if you want Tilt to update containers without rebuilding images
    live_update=[
        sync('./main.go', '/src/main.go'),
        sync('./pkg', '/src/pkg')
    ]
    #     run('go build -o app ./src'),  # Adjust according to your actual build command
    #     restart_container()
)

k8s_yaml(helm("./kubernetes/helm", set=[
    "server.enabled=true",
    "server.resources.limits.memory=1024Mi",
    "server.securityContext.runAsUser=0",
    "server.securityContext.runAsGroup=0",
    "server.securityContext.runAsNonRoot=false",
    "server.containerSecurityContext.readOnlyRootFilesystem=false",
    "server.containerSecurityContext.privileged=true",
    "server.containerSecurityContext.allowPrivilegeEscalation=true",
    "docker.image=wormhole",
    "docker.registry=",
    "devMode.enabled=true",
]))