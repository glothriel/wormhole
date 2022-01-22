# Wormhole

Reverse tunnels over websocket, similar to ngrok. Not yet production ready.

## Usage

### Client

```
wormhole mesh join --as piwikpro --expose name=python-server,address=127.0.0.1:1234 --expose 127.0.0.1:4321
```

### Server

```
wormhole mesh join --as piwikpro --expose name=python-server,address=127.0.0.1:1234 --expose 127.0.0.1:4321
```
