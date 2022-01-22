package server

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func newSessionHandler(
	peer peers.Peer,
	session *connection,
	appName string,
) *sessionHandler {
	theHandler := &sessionHandler{
		peer:    peer,
		session: session,
		appName: appName,
	}
	return theHandler
}

type messageRouter interface {
	Get(string) chan messages.Message
	Done(string)
}

// sessionHandler is responsible for passing messages between the peer and port opened for the app
type sessionHandler struct {
	peer    peers.Peer
	session *connection
	appName string
}

func (sh *sessionHandler) handleIncomingPeerMessages(router messageRouter) {
	logger := logrus.WithField("session_id", sh.session.id).WithField("peer_id", sh.peer.Name())
	defer func() {
		logger.Debugf(
			"Finished HandleIncomingPeerMessages",
		)
	}()
	for message := range router.Get(sh.session.id) {
		logger.Infof("Message came from peer")
		if messages.IsFrame(message) {
			if writeErr := sh.session.write(message); writeErr != nil {
				logrus.Fatal(writeErr)
			}
			logger.Infof("Message sent to session")
		}
	}
}

func (sh *sessionHandler) handleIncomingServiceMessages(router messageRouter) {
	defer router.Done(sh.session.id)
	logger := logrus.WithField("session_id", sh.session.id).WithField("peer_id", sh.peer.Name())
	defer func() {
		logger.Debugf(
			"Finished HandleIncomingServiceMessages",
		)
	}()
	for {
		downstreamMsg, receiveErr := sh.session.receive()
		if receiveErr != nil {
			if errors.Is(receiveErr, io.EOF) {
				logger.Debug("Session connection terminated")
				return
			}
			logrus.Error(receiveErr)
			return
		}

		logger.Debug("Message came from session")

		if sendErr := sh.peer.Send(
			messages.WithAppName(downstreamMsg, sh.appName),
		); sendErr != nil {
			logrus.Error(sendErr)
			return
		}

		logger.Debug("Message sent to peer")
	}
}

func (sh *sessionHandler) Handle(router messageRouter) {
	go sh.handleIncomingPeerMessages(router)
	go sh.handleIncomingServiceMessages(router)
}

// perAppPortExposer exposes a port for every app and allows retrieving new connections for every app
type perAppPortExposer struct {
	appName  string
	listener net.Listener
	port     int

	sessions sync.Map
}

func (sm *perAppPortExposer) connections() (chan *connection, error) {
	theChan := make(chan *connection)
	go func(theChan chan *connection) {
		defer func() { close(theChan) }()
		for {
			tcpC, acceptErr := sm.listener.Accept()
			if acceptErr != nil {
				if !errors.Is(acceptErr, net.ErrClosed) {
					logrus.Errorf("Failed to accept new TCP connection: %s", acceptErr)
				}
				return
			}
			ID := uuid.New().String()[:6]
			logrus.Infof(
				"New session ID %s", ID,
			)
			theSession := &connection{
				upstreamConn: tcpC,
				id:           ID,
				port:         sm.port,
			}
			sm.sessions.Store(ID, theSession)
			theChan <- theSession
		}
	}(theChan)

	return theChan, nil
}

func (sm *perAppPortExposer) terminate() error {
	return sm.listener.Close()
}

func newPerAppPortExposer(name string, allocator PortAllocator) (*perAppPortExposer, error) {
	var listener net.Listener
	var freePort int
	if retryErr := retry.Do(
		func() error {
			var portErr error
			freePort, portErr = allocator.GetFreePort()
			if portErr != nil {
				return portErr
			}
			address := fmt.Sprintf("localhost:%d", freePort)

			var listenErr error
			logrus.Infof("Estabilished new session manager on %s for %s", address, name)
			listener, listenErr = net.Listen("tcp", address)
			return listenErr
		},
		retry.Attempts(20),
		retry.Delay(time.Millisecond*10),
	); retryErr != nil {
		return nil, fmt.Errorf("Could not obtain a free port and start listening: %w", retryErr)
	}

	return &perAppPortExposer{
		appName:  name,
		listener: listener,
		port:     freePort,
	}, nil
}

// connection is a wrapper over TCP connection
type connection struct {
	id           string
	upstreamConn net.Conn
	port         int
}

func (s *connection) receive() (messages.Message, error) {
	buf := make([]byte, 64*1024)
	readBytes, readErr := s.upstreamConn.Read(buf)
	if readErr != nil {
		return messages.Message{}, fmt.Errorf("Failed to read from TCP connection %w", readErr)
	}
	return messages.NewFrame(s.id, buf[:readBytes]), nil
}

func (s *connection) write(m messages.Message) error {
	_, writeErr := s.upstreamConn.Write(messages.Body(m))
	if writeErr != nil {
		return fmt.Errorf("Failed to write to TCP connection %w", writeErr)
	}
	return writeErr
}
