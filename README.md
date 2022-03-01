# Wormhole

Reverse tunnels over websocket, similar to ngrok, teleport or skupper. Not production ready, this is mostly a learning project.

## Usage

Really, you shouldn't use this ESPECIALLY anywhere near production.

### Client

```
wormhole mesh join --as piwikpro --expose name=python-server,address=127.0.0.1:1234
```

### Server

```
wormhole mesh listen
```
