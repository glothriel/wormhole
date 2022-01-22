package peers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const dataExchangeEndpoint = "/data"

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type websocketPeer struct {
	PeerName   string
	Connection *websocket.Conn

	writeChan chan writeChanRequest
	readChans []chan messages.Message
	theApps   []App

	callbacks []func()

	logger *logrus.Entry
}

type writeChanRequest struct {
	message messages.Message
	errChan chan error
}

func (wt *websocketPeer) Send(message messages.Message) error {
	waitErr := make(chan error)
	wt.writeChan <- writeChanRequest{
		message: message,
		errChan: waitErr,
	}
	theErr := <-waitErr
	close(waitErr)
	return theErr
}

func (wt *websocketPeer) sendWorker() {
	wt.logger.Info("Started concurrent websocket sending worker")
	defer func() {
		wt.logger.Info("Stopped concurrent websocket sending worker")
	}()
	for request := range wt.writeChan {
		theBytes := messages.SerializeBytes(request.message)
		logrus.Infof("Sent %d bytes via websocket", len(theBytes))
		writeErr := wt.Connection.WriteMessage(websocket.BinaryMessage, theBytes)
		if writeErr != nil {
			request.errChan <- fmt.Errorf("Failed writing message to websocket: %w", writeErr)
		}
		wt.logger.Tracef(
			"Wrote websocket message %s", messages.Serialize(request.message),
		)
		request.errChan <- nil
	}
}

func (wt *websocketPeer) Close() error {
	for _, readChan := range wt.readChans {
		close(readChan)
	}
	close(wt.writeChan)
	closeErr := wt.Connection.Close()
	if closeErr != nil {
		return fmt.Errorf("Failed closing websocket connection: %w", closeErr)
	}
	for _, cb := range wt.callbacks {
		cb()
	}
	return nil
}

func (wt *websocketPeer) Name() string {
	return wt.PeerName
}

func (wt *websocketPeer) Receive() (chan messages.Message, error) {
	theChannel := make(chan messages.Message)
	wt.readChans = append(wt.readChans, theChannel)
	go func() {
		defer func() {
			wt.logger.Info("Websocket Receive goroutine shutdown")
		}()
		wt.logger.Infof("Ready for receiving websocket messages from %s", wt.Connection.RemoteAddr().String())
		for {
			_, msg, readMessageErr := wt.Connection.ReadMessage()
			if readMessageErr != nil {
				if !websocket.IsUnexpectedCloseError(readMessageErr) {
					logrus.Error(readMessageErr)
				}
				wt.Close()
				return
			}
			theMsg, deserializeErr := messages.DeserializeMessageBytes(msg)
			if deserializeErr != nil {
				logrus.Error(deserializeErr)
				continue
			}
			wt.logger.Tracef(
				"Got websocket message %s", messages.Serialize(theMsg),
			)
			theChannel <- theMsg
		}
	}()
	return theChannel, nil
}

func (wt *websocketPeer) Apps() ([]App, error) {
	return wt.theApps, nil
}

func (wt *websocketPeer) WhenClosed(cb func()) {
	wt.callbacks = append(wt.callbacks, cb)
}

// NewWebsocketPeer creates new websocketPeer instances, that implement Peer over a websocket connection
func NewWebsocketPeer(
	name string,
	connection *websocket.Conn,
	upstreams []App,

) Peer {
	peer := &websocketPeer{
		PeerName:   name,
		Connection: connection,
		writeChan:  make(chan writeChanRequest),
		theApps:    upstreams,
		logger:     logrus.WithField("peer", name),
	}
	go peer.sendWorker()
	return peer
}

// NewWebsocketClientPeer allows creating peers that are clients, connecting to remote servers
func NewWebsocketClientPeer(peerName, serverAddr string, apps []App) (Peer, error) {
	protocol, theAddr, parseErr := parseServerAddr(serverAddr)
	if parseErr != nil {
		return nil, parseErr
	}
	queryConfig, marshalErr := json.Marshal(apps)
	if marshalErr != nil {
		return nil, marshalErr
	}
	u := url.URL{
		Scheme:   protocol,
		Host:     theAddr,
		Path:     dataExchangeEndpoint,
		RawQuery: fmt.Sprintf("upstreams=%s&name=%s", string(queryConfig), peerName),
	}

	c, _, dialErr := websocket.DefaultDialer.Dial(u.String(), nil)
	if dialErr != nil {
		return nil, dialErr
	}
	thePeer := NewWebsocketPeer(peerName, c, apps)
	return thePeer, nil
}

type websocketServerPeerFactory struct {
	peers chan Peer
}

func (wsTransportFactory *websocketServerPeerFactory) Peers() (chan Peer, error) {
	return wsTransportFactory.peers, nil
}

// NewWebsocketServerPeerFactory allows creating peers, that are servers, waiting for clients to connect to them
func NewWebsocketServerPeerFactory(host, port string) (PeerFactory, error) {
	peersChan := make(chan Peer)
	router := mux.NewRouter()
	router.HandleFunc(dataExchangeEndpoint,
		func(w http.ResponseWriter, r *http.Request) {
			var configuredUpstreams []App
			unmarshalErr := json.Unmarshal([]byte(r.URL.Query().Get("upstreams")), &configuredUpstreams)
			if unmarshalErr != nil {
				logrus.Error(unmarshalErr)
				return
			}

			websocketConnection, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Print("upgrade:", err)
				return
			}

			thePeer := NewWebsocketPeer(
				r.URL.Query().Get("name"),
				websocketConnection,
				configuredUpstreams,
			)

			peersChan <- thePeer
		})

	http.Handle("/", router)
	serverAddr := fmt.Sprintf("%s:%s", host, port)
	logrus.Info(fmt.Sprintf("Starting HTTP server at %s", serverAddr))
	go func() {
		logrus.Info(http.ListenAndServe(serverAddr, router))
	}()

	return &websocketServerPeerFactory{
		peers: peersChan,
	}, nil
}

func parseServerAddr(serverAddr string) (string, string, error) {
	re := regexp.MustCompile(`(wss|ws):\/\/([a-z\.0-9]*:\d*)`)
	matches := re.FindAllStringSubmatch(serverAddr, -1)
	if len(matches) == 0 {
		return "", "", fmt.Errorf("Invalid server address, must match %s, received %s", re.String(), serverAddr)
	}
	return matches[0][1], matches[0][2], nil
}
