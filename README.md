# Wormhole

L3 (wireguard) and L4 (NGINX) reverse TCP tunnels over wireguard, similar to ngrok, teleport or skupper, but implemented specifically for Kubernetes. Mostly a learning project. Allows exposing services from one Kubernetes cluster to another just by annotating them.

Wormhole is implemented using "Hub and spoke" architecture. One cluster acts as a central hub, while others are clients. Clients can expose services to the hub and the hub can expose services to the clients. Exposing of the services between the clients is **not supported**.

## Architecture

Wormhole uses a combination of three components in order to work:

* Wormhole controller - a Kubernetes controller that watches for services with a specific annotation and creates tunnels for them
* Bundled Nginx - a simple Nginx server that is dynamically configured to proxy requests to the tunnels and vice versa
* Wireguard - a VPN server that is used to create secure tunnels between the clusters, it's also dynamically configured by the controller

This repository contains source code for all of the components.

![](./docs/overview.jpg)

### Peering

Peering is the process of establishing a connection between two clusters. The peering is performed outside of the tunnel, using the HTTP API exposed by the server over the public internet. The peering by default is performed using HTTP protocol, but you may put the server behind SSL-terminating reverse proxy. Saying that, the communication is encrypted using a PSK, that both the client and server must know prior to the peering. The communication goes as follows.

* Operator deploys the server and client, configuring them with the same PSK, for example `supersecret`
* Upon startup, the client continuously tries to connect to the server using the HTTP API, encrypting the payload of the request with the PSK
* The server knowing the PSK, decrypts the payload and checks if the client is allowed to connect. If it's not able to decrypt the payload, it means that the client doesn't know the PSK and the connection is rejected.
* The peering message looks something like this (of course as stated above it's encrypted):
    * `{"name": "client1", "wireguard": {"public_key": "xyz...xyz"}, "metadata": {}}`
* The server checks if declared name matches the public key (if there were previous successful peerings) or creates a new client with the given name and public key. It also updates its Wireguard configuration with the new client and responds with something like this (of course encrypted):
    * `{"name": "server", "wireguard": {"public_key": "abc...abc"}, "metadata": {}, "assigned_ip": "192.168.11.6", "internal_server_ip": "192.168.11.1"}`
    * The response is similar, but contains also the server's public key, assigned IP for the client, and the server's internal (VPN) IP address.
* The client updates its Wireguard configuration with the server's public key and the assigned IP, and starts the Wireguard tunnel. The tunnel is now established and the client can communicate with the server.

### Syncing

Syncing is a process of exchanging information about exposed applications on both client and server. The syncing is performed over the Wireguard tunnel, so it's secure. The syncing goes as follows:
* Both client and server observe the state of kubernetes services deployed on their respective clusters. If a service is annotated with `wormhole.glothriel.github.com/exposed=yes`, it's considered exposed and added to internal exposed apps registry.
* Every 5 (configurable) seconds, the client performs a HTTP request over the wireguard tunnel to the server, sending the list of exposed services. It looks like this:
    * `{"peer": "client1", "apps": [{"name": "nginx", "address": "192.168.1.6:25001", "original_port" :80}]}`
* The response from the server is exactly the same, but contains the list of exposed services on the server side. The client updates its internal registry with the server's exposed services, both create nginx proxies and respective kubernetes services for the apps exposed by the opposite side.


## Usage

You can install wormhole using helm. For server you will need a cluster with LoadBalancer support, for client - any cluster. IP exposed by the server's LoadBalancer must be reachable from the client's cluster.

You can optionally install both the server and the client on the same cluster and use ClusterIP service for communication. See the [./Tiltfile](./Tiltfile) for an example, as the development environment uses this approach.

### Install server

Server is a central component of wormhole. It allows clients to connect and hosts the tunnels. It exposes two services:

* HTTP API for peering (initial peering is performed outside of the tunnel)
* Wireguard server for tunnel

If you'll use DNS, you can install the server in one step (replace 0.0.0.0 with the public hostname), otherwise you'll have to wait for the LoadBalancer to get an IP and update configuration after that.

```
kubectl create namespace wormhole

# Replace 1.0.0 with latest version from the releases page
helm install -n wormhole wh oci://ghcr.io/glothriel/wormhole/wormhole --version 1.0.0 --set server.enabled=true --set server.service.type=LoadBalancer --set server.wg.publicHost="0.0.0.0"

# Wait for the LoadBalancer to get an IP
kubectl get svc -n wormhole

# Update the server with the IP
helm upgrade -n wormhole wh oci://ghcr.io/glothriel/wormhole/wormhole --version 1.0.0 --set server.enabled=true --set server.service.type=LoadBalancer --set server.wg.publicHost="<the new IP>"
```

### Install client

You should do this on another cluster. If not, change the namespace to say `wormhole-client` to avoid conflicts. Please note the `client.name` parameter - it should be unique for each client. At this point you may add as many clients as you want.

```
kubectl create namespace wormhole

helm install -n wormhole wh kubernetes/helm --set client.enabled=true --set client.serverDsn="http://<server.wg.publicHost>:8080" --set client.name=client-one
```

### Expose a service

Now you can expose a service from one infrastructure to another. Services exposed from the server will be available on all the clients. Services exposed from the client will be available only on the server.

```
kubectl annotate --overwrite svc --namespace <namespace> <service> wormhole.glothriel.github.com/exposed=yes
```

After up to 30 seconds the service will be available on the other side. 

### Customize the exposed services

You can use two additional annotations to customize how the service is exposed on the other side:

```
# Customize the service name
wormhole.glothriel.github.com/name=my-custom-name

# If the service uses more than one port, you can specify which ports should be exposed
wormhole.glothriel.github.com/ports=http
wormhole.glothriel.github.com/ports=80,443
```

## Local development

### Development environment

Requirements:

* Helm
* Tilt
* K3d

```
k3d cluster create wormhole --registry-create wormhole

tilt up
```

First start of wormhole will be really slow - it compiles the go code inside the container. Subsequent starts will be faster, as the go build cache is preserved in PVC.

The development environment deploys a server, two clients and a mock service, that you can use to test the tunnels.

```
kubectl annotate --overwrite svc --namespace nginx nginx  wormhole.glothriel.github.com/exposed=yes
```

The additional services should be immediately created. Please note, that all three workloads are deployed on the same (and by extension are monitoring the same services for annotations), so the nginx will be exposed 4 times - client1 to server, client2 to server, server to client1 and server to client2.

### Integration tests

```
cd tests && python setup.py develop && cd -

pytest tests
```