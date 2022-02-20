package peers

import (
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

type writeChanRequest struct {
	message messages.Message
	errChan chan error
}

func parseServerAddr(serverAddr string) (string, string, error) {
	re := regexp.MustCompile(`(wss|ws):\/\/([a-z\.0-9]*:\d*)`)
	matches := re.FindAllStringSubmatch(serverAddr, -1)
	if len(matches) == 0 {
		return "", "", fmt.Errorf("Invalid server address, must match %s, received %s", re.String(), serverAddr)
	}
	return matches[0][1], matches[0][2], nil
}

type websocketTransport struct {
	PeerName   string
	Connection *websocket.Conn

	writeChan chan writeChanRequest
	readChans []chan messages.Message
}

func (transport *websocketTransport) Send(message messages.Message) error {
	waitErr := make(chan error)
	transport.writeChan <- writeChanRequest{
		message: message,
		errChan: waitErr,
	}
	theErr := <-waitErr
	close(waitErr)
	return theErr
}

func (transport *websocketTransport) sendWorker() {
	for request := range transport.writeChan {
		theBytes := messages.SerializeBytes(request.message)
		writeErr := transport.Connection.WriteMessage(websocket.BinaryMessage, theBytes)
		if writeErr != nil {
			request.errChan <- fmt.Errorf("Failed writing message to websocket: %w", writeErr)
		} else {
			request.errChan <- nil
		}
	}
}

func (transport *websocketTransport) Receive() (chan messages.Message, error) {
	theChannel := make(chan messages.Message)
	transport.readChans = append(transport.readChans, theChannel)
	go func() {
		for {
			_, msg, readMessageErr := transport.Connection.ReadMessage()

			if readMessageErr != nil {
				if !websocket.IsUnexpectedCloseError(readMessageErr) {
					logrus.Error(readMessageErr)
				}
				transport.Close()
				return
			}
			theMsg, deserializeErr := messages.DeserializeMessageBytes(msg)
			if deserializeErr != nil {
				logrus.Error(deserializeErr)
				continue
			}
			theChannel <- theMsg
		}
	}()
	return theChannel, nil
}

func (transport *websocketTransport) Close() error {
	for _, readChan := range transport.readChans {
		close(readChan)
	}
	close(transport.writeChan)
	closeErr := transport.Connection.Close()
	if closeErr != nil {
		return fmt.Errorf("Failed closing websocket connection: %w", closeErr)
	}
	return nil
}

// NewWebsocketTransport creates new websocketTransport instances, that implement Transport over a websocket connection
func NewWebsocketTransport(
	connection *websocket.Conn,

) Transport {
	peer := &websocketTransport{
		Connection: connection,
		writeChan:  make(chan writeChanRequest),
	}
	go peer.sendWorker()
	return peer
}

// NewWebsocketClientTransport creates new websocketTransport instances, that implement Transport over a websocket
func NewWebsocketClientTransport(
	serverAddr string,
) (Transport, error) {
	protocol, theAddr, parseErr := parseServerAddr(serverAddr)
	if parseErr != nil {
		return nil, parseErr
	}
	u := url.URL{
		Scheme: protocol,
		Host:   theAddr,
		Path:   dataExchangeEndpoint,
	}

	c, _, dialErr := websocket.DefaultDialer.Dial(u.String(), nil)
	if dialErr != nil {
		return nil, dialErr
	}

	peer := &websocketTransport{
		Connection: c,
		writeChan:  make(chan writeChanRequest),
	}
	go peer.sendWorker()
	return peer, nil
}

type websocketTransportFactory struct {
	transports chan Transport
}

func (wsTransportFactory *websocketTransportFactory) Create() (Transport, error) {
	return <-wsTransportFactory.transports, nil
}

// NewWebsocketTransportFactory allows creating peers, that are servers, waiting for clients to connect to them
func NewWebsocketTransportFactory(host, port string) (TransportFactory, error) {
	transportsChan := make(chan Transport)
	router := mux.NewRouter()
	router.HandleFunc(dataExchangeEndpoint,
		func(w http.ResponseWriter, r *http.Request) {
			websocketConnection, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Print("upgrade:", err)
				return
			}

			thePeer := NewWebsocketTransport(
				websocketConnection,
			)

			transportsChan <- thePeer
		})

	http.Handle("/", router)
	serverAddr := fmt.Sprintf("%s:%s", host, port)
	logrus.Info(fmt.Sprintf("Starting HTTP server at %s", serverAddr))
	go func() {
		logrus.Info(http.ListenAndServe(serverAddr, router))
	}()

	return &websocketTransportFactory{
		transports: transportsChan,
	}, nil
}
