# Wormhole

L4 reverse TCP tunnels over wireguard, similar to ngrok, teleport or skupper, but implemented specifically for Kubernetes. Mostly a learning project. Allows exposing services from one Kubernetes cluster to another just by annotating them.

## Helm

You can install wormhole using helm. Please clone this repository first. For server you will need a cluster with LoadBalancer support, for client - any cluster. IP exposed by the server's LoadBalancer must be reachable from the client's cluster.

### Install server

Server is a central component of wormhole. It allows clients to connect and hosts the tunnels. It exposes two services:

* HTTP API for peering (initial peering is performed outside of the tunnel)
* Wireguard server for tunnel

If you'll use DNS, you can install the server in one step (replace 0.0.0.0 with the public hostname), otherwise you'll have to wait for the LoadBalancer to get an IP and update configuration after that.

```
kubectl create namespace wormhole

helm install -n wormhole wh kubernetes/helm --set server.enabled=true --set server.service.type=LoadBalancer --set server.wg.publicHost="0.0.0.0"

# Wait for the LoadBalancer to get an IP
kubectl get svc -n wormhole

# Update the server with the IP
helm upgrade -n wormhole wh kubernetes/helm --set server.enabled=true --set server.service.type=LoadBalancer --set server.wg.publicHost="<the new IP>"
```

### Install client

You should do this on another cluster. If not, change the namespace to say `wormhole-client` to avoid conflicts.

```
kubectl create namespace wormhole

helm install -n wormhole wh kubernetes/helm --set client.enabled=true --set client.serverDsn="http://server.wg.publicHost:8080"
```

### Expose a service

No you can expose a service from one infrastructure to another. Services exposed from the server will be available on all the clients. Services exposed from the client will be available only on the server.

```
kubectl annotate --overwrite svc <namespace> <service> wormhole.glothriel.github.com/exposed=yes
```

After up to 30 seconds the service will be available on the other side. You can check the status of the tunnel by running:

