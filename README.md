# Wormhole

L7 reverse TCP tunnels over websocket, similar to ngrok, teleport or skupper, but implemented specifically for Kubernetes. Mostly a learning project. Allows exposing services from one Kubernetes cluster to another just by annotating them.

![overview](docs/overview.jpg "Overview")

## What should I use this for?

To be honest, currently you should consider using something like Gravitational Teleport or Hashicorp Boundary. This project was started, because I was not satisfied with Teleport, due to:
* The nodes forgetting about each other all the time and the need of manual re-connecting them by re-generating invite tokens
* The proxied sockets on Teleport require SSL certs, that you need to generate by Teleport API. My central cluster is trusted, so this was unnecessary complication and made configuring some integrations I cared about harder and almost impossible to automate.
* No plug-in integration with kubernetes like wormhole has (just annotate the service and it's automatically mirrored on central cluster)

Boundary, when I evaluated it, didn't support reverse tunnels (expected port to be opened on edge infras), so it was out of question.

## Helm

You can install wormhole using helm. Please clone this repository first.

### Install server

Server must expose port (container port 8080) for the clients that will be creating reverse tunnels. The port may be exposed directly with service type LoadBalancer (set `server.service.type` to `LoadBalancer`), or you can put wormhole behind Ingress Controller (tested with HAProxy), as it's basically a websocket server. In that case, you need to provide your Ingress resource yourself.

The below commands assume, that you are deploying everything in single cluster just to test things, so it does not care about exposing 8080 port externally at all.

```
kubectl create namespace wormhole-server
helm install -n wormhole-server whserver kubernetes/helm --set server.enabled=true
```

### Install client

This command allows installing client in the same cluster as server, for testing purposes. If you'd like to deploy client in other cluster, please adjust `client.serverDsn`.

```
kubectl create namespace wormhole-client
helm install -n wormhole-client whclient kubernetes/helm --set client.name=testclient --set client.enabled=true --set client.serverDsn=ws://wormhole-server-whserver.wormhole-server:8080/wh/tunnel
```

### Approve pairing request

Client when connects to server generates a RSA key pair (in-depth description below in "Authorization & SSL" section). You need to tell the server, that the client is trusted in order for them to start exchanging messages.

In order to do that, review the client logs:
```
kubectl logs -n wormhole-client deployment/wormhole-client-whclient
```

You should see something like this:
```
INFO[0000] Log level set to info
INFO[0000] Sending public key to the server, please make sure, that the fingerprint matches: <FINGERPRINT>
```

Please copy the fingerprint to clipboard and accept the connection request using the CLI:

```
kubectl exec -n wormhole-server deployment/wormhole-server-whserver -- wormhole requests accept <FINGERPRINT>
```

Now the client and server are deployed and paired, you can start annotating services.

```
kubectl -n default annotate svc kubernetes wormhole.glothriel.github.com/exposed=yes
```

A proxy service should be created in namespace `wormhole-server`: `testclient-default-kubernetes`. All the TCP connections made to the proxy service will be tunelled between the server and client to the destination service.

## APIs

### Client annotation API

You can expose a service that is deployed on Client's cluster by annotating it. Here are the annotations you can use:

| Annotation                            | Purpose                                                                                                                           | Example value                     |
|---------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------|-----------------------------------|
| wormhole.glothriel.github.com/exposed | Marks a service as exposed. By default all the ports will be exposed on the destination, as separate apps - so separate services. | "1", "yes", "true", "no", "false" |
| wormhole.glothriel.github.com/name    | Name under which the app will be exposed. If the annotation is not present, the name of the service is used.                      | "prometheus", "loki", "my-app"    |
| wormhole.glothriel.github.com/ports   | List of ports for given service, that should be exposed. Can use both names and numbers, remember to use strings.                 | "metrics", "1337", "web"          |

### Server Admin API

Server admin API is exposed on port 8081.

#### GET /v1/apps

Returns list of apps exposed by connected clients.

**Example response**:

```
HTTP 200
Content-Type: application/json

[
    {
        "app": "prometheus",
        "endpoint": "prometheus-infraone.wormhole-server:8080",
        "peer": "infraone"
    },
    {
        "app": "prometheus",
        "endpoint": "prometheus-infratwo.wormhole-server:8080",
        "peer": "infratwo"
    }
]
```

#### GET /v1/requests

Displays list of pairing requests - fingerprints only.

**Example response**:

```
HTTP 200
Content-Type: application/json

[
    "231::46::1::217::196",
    "5::12::142::62::4",
]
```

#### POST /v1/requests/{fingerprint}

Accepts pairing requests

**Example response**:

```
HTTP 204
```

#### DELETE /v1/requests/{fingerprint}

Declines pairing requests

**Example response**:

```
HTTP 204
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
docker.version | It's advised to change this to a tag | latest

### server

Parameter | Description | Default
----------|-------------|---------
server.acceptor | Set to "dummy" to automatically accept all clients | server
server.affinity | | None
server.containerSecurityContext.allowPrivilegeEscalation | | False
server.containerSecurityContext.privileged | | False
server.containerSecurityContext.readOnlyRootFilesystem | | True
server.enabled | | False
server.nodeSelector | | None
server.path | HTTP path under which the tunnel is opened. If empty uses default from CLI (`/wh/tunnel`) | ""
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

## FAQ

**Is UDP supported**
No. Maybe someday.

**How quick it is?**
It's super slow. It's websockets. The tunnel code itself is closer to a POC than production solution - a lot of allocations, conversions between byte-arrays to strings, encodings, etc. This will definitely be improved once core functionality is finished up, but please note, that very high performance will never be the goal of this project.

**Why websockets?**
Stubborn on-prem clients are easier to persuade to open an outbound port to a 443 web server, than a random TCP socket. As funny as it seems, this is really the reason.

**Is exposing services from server to client possible?**
Currently - no. In the future, if i have enough determination - yes.

## Development

```k3d cluster create wormhole --registry-create wormhole```
