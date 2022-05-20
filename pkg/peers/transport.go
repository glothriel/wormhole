package peers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// Transport is used to allow communication between the peers
type Transport interface {
	Send(messages.Message) error
	Receive() (chan messages.Message, error)
	Close() error
}

// TransportFactory creates Transport instances
type TransportFactory interface {
	Transports() (chan Transport, error)
}

// MockTransport implements Transport and can be used for unit tests
type MockTransport struct {
	theOtherOne *MockTransport
	inbox       chan messages.Message
	closed      bool
}

// Send implements Transport
func (transport *MockTransport) Send(message messages.Message) error {
	transport.theOtherOne.inbox <- message
	return nil
}

// Receive implements Transport
func (transport *MockTransport) Receive() (chan messages.Message, error) {
	return transport.inbox, nil
}

// Close implements Transport
func (transport *MockTransport) Close() error {
	transport.closed = true
	close(transport.inbox)
	return nil
}

// CreateMockTransportPair creates two mock transports
func CreateMockTransportPair() (*MockTransport, *MockTransport) {
	first := &MockTransport{
		inbox: make(chan messages.Message, 255),
	}
	second := &MockTransport{
		inbox:       make(chan messages.Message, 255),
		theOtherOne: first,
	}
	first.theOtherOne = second
	return first, second
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type wsWriteChanRequest struct {
	message messages.Message
	errChan chan error
}

type websocketTransport struct {
	PeerName   string
	Connection *websocket.Conn

	writeChan chan wsWriteChanRequest
	readChans []chan messages.Message
}

func (transport *websocketTransport) Send(message messages.Message) (theErr error) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Trace("Recovered panic when trying to send to closed channel in websocket transport")
		}
	}()
	// The default value is returned when above panic is triggered
	theErr = errors.New("The connection is closed, message cannot be sent")
	waitErr := make(chan error)
	transport.writeChan <- wsWriteChanRequest{
		message: message,
		errChan: waitErr,
	}
	// If the panic did not trigger, the default is overridden
	theErr = <-waitErr
	close(waitErr)
	return theErr
}

func (transport *websocketTransport) sendWorker() {
	for request := range transport.writeChan {
		theBytes := messages.SerializeBytes(request.message)
		logrus.Debugf("Sending message: %s", string(theBytes))
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
				if closeErr := transport.Close(); closeErr != nil {
					if !strings.Contains(closeErr.Error(), "use of closed network connection") {
						logrus.Warnf("Failed to close transport: %s", closeErr)
					}
				}
				return
			}
			logrus.Debugf("Received message: %s", string(msg))
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
		forceClose(readChan)
	}
	forceClose(transport.writeChan)
	closeErr := transport.Connection.Close()
	if closeErr != nil {
		return fmt.Errorf("Failed closing websocket connection: %w", closeErr)
	}
	return nil
}

func forceClose[V chan messages.Message | chan wsWriteChanRequest](c V) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Debugf("Recovered in %s", r)
		}
	}()
	close(c)
}

// NewWebsocketTransport creates new websocketTransport instances, that implement Transport over a websocket connection
func NewWebsocketTransport(
	connection *websocket.Conn,

) Transport {
	peer := &websocketTransport{
		Connection: connection,
		writeChan:  make(chan wsWriteChanRequest),
	}
	go peer.sendWorker()
	return peer
}

// NewWebsocketClientTransport creates new websocketTransport instances, that implement Transport over a websocket
func NewWebsocketClientTransport(
	serverAddr string,
) (Transport, error) {
	URL, parseErr := url.Parse(serverAddr)
	if parseErr != nil {
		return nil, parseErr
	}
	u := url.URL{
		Scheme: URL.Scheme,
		Host:   URL.Host,
		Path:   URL.Path,
	}
	c, _, dialErr := websocket.DefaultDialer.Dial(u.String(), nil)
	if dialErr != nil {
		return nil, dialErr
	}

	peer := &websocketTransport{
		Connection: c,
		writeChan:  make(chan wsWriteChanRequest),
	}
	go peer.sendWorker()
	return peer, nil
}

type websocketTransportFactory struct {
	transports chan Transport
}

func (wsTransportFactory *websocketTransportFactory) Transports() (chan Transport, error) {
	return wsTransportFactory.transports, nil
}

// NewWebsocketTransportFactory allows creating peers, that are servers, waiting for clients to connect to them
func NewWebsocketTransportFactory(host, port, path string) (TransportFactory, error) {
	transportsChan := make(chan Transport)
	router := mux.NewRouter()
	router.HandleFunc(path,
		func(w http.ResponseWriter, r *http.Request) {
			websocketConnection, err := wsUpgrader.Upgrade(w, r, nil)
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

// aesTransport is a decorator over Transport interface, that encrypts all the messages in transit
type aesTransport struct {
	child    Transport
	password string
}

// CiphertextTag prefixes all messages that have body encrypted
const CiphertextTag = "AesCiphertext::"

// Send implements Transport
func (transport *aesTransport) Send(message messages.Message) error {
	cipherText, encryptErr := encrypt(
		transport.password, []byte(message.BodyString),
	)
	if encryptErr != nil {
		return encryptErr
	}
	return transport.child.Send(
		messages.WithBody(
			message,
			strings.Join([]string{CiphertextTag, base64.RawStdEncoding.EncodeToString(cipherText)}, ""),
		),
	)
}

// Receive implements Transport
func (transport *aesTransport) Receive() (chan messages.Message, error) {
	localChan := make(chan messages.Message)

	childChan, childReceiveErr := transport.child.Receive()
	if childReceiveErr != nil {
		return nil, childReceiveErr
	}
	go func() {
		for remoteMessage := range childChan {
			encryptedBase64, base64Err := base64.RawStdEncoding.DecodeString(remoteMessage.BodyString[len(CiphertextTag):])
			if base64Err != nil {
				logrus.Errorf("Could not decode base64: %v", base64Err)
			}

			plainText, decryptErr := decrypt(
				transport.password, encryptedBase64,
			)
			if decryptErr != nil {
				logrus.Errorf("Could not decrypt BodyString of incoming message: %v", decryptErr)
				// continue
			}
			localChan <- messages.WithBody(remoteMessage, string(plainText))
		}
		close(localChan)
	}()
	return localChan, nil
}

// Close implements Transport
func (transport *aesTransport) Close() error {
	return transport.child.Close()
}

// NewAesTransport creates AesTranport instances
func NewAesTransport(password string, child Transport) Transport {
	return &aesTransport{
		password: password,
		child:    child,
	}
}

type aesTransportFactory struct {
	password   string
	child      TransportFactory
	transports chan Transport
}

func (factory *aesTransportFactory) Transports() (chan Transport, error) {
	myTransports := make(chan Transport)
	childTransports, transportErr := factory.child.Transports()
	if transportErr != nil {
		return nil, transportErr
	}
	go func() {
		for childTransport := range childTransports {
			myTransports <- &aesTransport{
				password: factory.password,
				child:    childTransport,
			}
		}
		close(myTransports)
	}()
	return myTransports, nil
}

// NewAesTransportFactory is a decorator over TransportFactory, that allows encryption in transit with AES
func NewAesTransportFactory(password string, child TransportFactory) TransportFactory {
	transportsChan := make(chan Transport)

	return &aesTransportFactory{
		transports: transportsChan,
		password:   password,
		child:      child,
	}
}
