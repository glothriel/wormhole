# Wormhole

L7 reverse tunnels over websocket, similar to ngrok, teleport or skupper, but implemented specifically for Kubernetes. Mostly a learning project. Allows exposing services from one Kubernetes cluster to another just by annotating them.

![overview](docs/overview.jpg "Overview")

## What should I use this for?

To be honest, currently you should consider using something like Gravitational Teleport or Hashicorp Boundary. This project was started, because I was not satisfied with Teleport, due to:
* Connection errors and the need of manual re-connecting the nodes
* The sockets on Teleport side are protected with SSL, even though my central K8s cluster is trusted
* Authorization model with invite tokens, that make onboarding new edge infrastructures harder
* No plug-in integration with kubernetes like wormhole has (just annotate the service and it's automatically mirrored on central cluster)

Boundary, when I evaluated it, didn't support reverse tunnels (expected port to be opened on edge infras), so it was out of question.

## Helm

You can install wormhole using helm. Please clone this repository first.

### Install server

Server must expose port (container port 8080) for the clients that will be creating reverse tunnels. The port may be exposed directly with service type LoadBalancer (set `server.service.type` to `LoadBalancer`), or you can put wormhole behind Ingress Controller (tested with HAProxy), as it's basically a websocket server. In that case, you need to provide your Ingress resource yourself.

```
kubectl create namespace wormhole-server
helm install -n wormhole-server whserver kubernetes/helm --set server.enabled=true
```

### Install client

This command allows installing client in the same cluster as server, for testing purposes. If you'd like to deploy client in other cluster, please adjust `client.serverDsn`.

```
kubectl create namespace wormhole-client
helm install -n wormhole-client wormhole-client kubernetes/helm --set client.name=testclient --set client.enabled=true --set client.serverDsn=ws://wormhole-whserver-server.wormhole-server:8080/wh/tunnel
```

## Authorization & SSL

Wormhole itself doesn't support SSL, but can be put behind SSL-terminating reverse proxy, like HAProxy or Nginx, as it's just a websocket server. Please remember to set `wss` protocol in `client.serverDsn` value, if doing so.

**Wormhole with its authorization module is safe to operate and encrypts all the traffic even without SSL**. Authorization flow for wormhole goes as follows:
1. Client checks if 2048 bit RSA key was previously generated, if not, generates new one.
2. Client calculates a fingerprint (hash) of the key and displays it to the console.
3. First message when client connects to the server includes RSA public key.
4. Server receives the public key, calculates the fingerprint and waits for the human operator to manually approve the connection request.
5. If human operator declines, the connection is closed. If human operator approves, server generates 32 bit AES key, encrypts it with client's RSA public key and sends the AES key back to the client.
6. All the subsequent messages are encrypted with that AES key.

Security limitations:
1. At the moment the RSA keys are not automatically rotated, you need to remove them from filesystem manually if you want them rotated (this forces new fingerprint, so you need to re-approve the key next time client connects to the server)
2. AES key is generated when client connects to the server, so if you want to re-generate the key, you need to force re-connection of the client.

The above flow was optimized to make onboarding new infrastructures a little bit easier. Normally you'd probably just use SSL and sign the client RSA keys beforehand, but you'd need to generate, sign and deliver the certs for each new infra.

## Helm chart

### client

Parameter | Description | Default
----------|-------------|---------
client.affinity | | None
client.containerSecurityContext.allowPrivilegeEscalation | | False
client.containerSecurityContext.privileged | | False
client.containerSecurityContext.readOnlyRootFilesystem | | True
client.enabled | | False
client.name | | ""
client.nodeSelector | | None
client.priorityClassName | | ""
client.pullPolicy | | Always
client.pvc.enabled | | False
client.pvc.storage | | 1Gi
client.pvc.storageClassName | | ""
client.resources.limits.cpu | | 0
client.resources.limits.memory | | 128Mi
client.resources.requests.cpu | | 0
client.resources.requests.memory | | 128Mi
client.securityContext.fsGroup | | 1337
client.securityContext.runAsGroup | | 1337
client.securityContext.runAsNonRoot | | True
client.securityContext.runAsUser | | 1337
client.serverDsn | | ws://wormhole-server:8080/wh/tunnel
client.tolerations | | None

### docker

Parameter | Description | Default
----------|-------------|---------
docker.image | | glothriel/wormhole
docker.registry | | ghcr.io
docker.version | | latest

### server

Parameter | Description | Default
----------|-------------|---------
server.acceptor | | server
server.affinity | | None
server.containerSecurityContext.allowPrivilegeEscalation | | False
server.containerSecurityContext.privileged | | False
server.containerSecurityContext.readOnlyRootFilesystem | | True
server.enabled | | False
server.nodeSelector | | None
server.priorityClassName | | ""
server.pullPolicy | | Always
server.pvc.enabled | | False
server.pvc.storage | | 1Gi
server.pvc.storageClassName | | ""
server.resources.limits.cpu | | 0
server.resources.limits.memory | | 128Mi
server.resources.requests.cpu | | 0
server.resources.requests.memory | | 128Mi
server.securityContext.fsGroup | | 1337
server.securityContext.runAsGroup | | 1337
server.securityContext.runAsNonRoot | | True
server.securityContext.runAsUser | | 1337
server.service.type | | ClusterIP
server.tolerations | | None
