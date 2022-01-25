package server

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
)

func newAppConnectionHandler(
	peer peers.Peer,
	app peers.App,
	appConnection appConnection,
) *appConnectionHandler {
	theHandler := &appConnectionHandler{
		peer:          peer,
		appConnection: appConnection,
		app:           app,
	}
	return theHandler
}

type messageRouter interface {
	Get(string) chan messages.Message
	Done(string)
}

// appConnectionHandler is responsible for passing messages between the peer and port opened for the app
type appConnectionHandler struct {
	peer          peers.Peer
	appConnection appConnection
	app           peers.App
}

func (handler *appConnectionHandler) handleIncomingPeerMessages(router messageRouter) {
	logger := logrus.WithField("session_id", handler.appConnection.sessionID).WithField("peer_id", handler.peer.Name())
	for message := range router.Get(handler.appConnection.sessionID()) {
		logger.Infof("Message came from peer")
		if messages.IsFrame(message) {
			if writeErr := handler.appConnection.write(message); writeErr != nil {
				logrus.Fatal(writeErr)
			}
			logger.Infof("Message sent to session")
		}
	}
}

func (handler *appConnectionHandler) handleIncomingAppMessages(router messageRouter) {
	defer router.Done(handler.appConnection.sessionID())
	logger := logrus.WithField("session_id", handler.appConnection.sessionID).WithField("peer_id", handler.peer.Name())
	for {
		downstreamMsg, receiveErr := handler.appConnection.receive()
		if receiveErr != nil {
			if errors.Is(receiveErr, io.EOF) {
				logger.Debug("Session connection terminated")
				return
			}
			logrus.Error(receiveErr)
			return
		}

		if sendErr := handler.peer.Send(
			messages.WithAppName(downstreamMsg, handler.app.Name),
		); sendErr != nil {
			logrus.Error(sendErr)
			return
		}
	}
}

func (handler *appConnectionHandler) Handle(router messageRouter) {
	go handler.handleIncomingPeerMessages(router)
	go handler.handleIncomingAppMessages(router)
}

// tcpAppConnection is a wrapper over TCP connection that implements appConnection
type tcpAppConnection struct {
	theSessionID string
	conn         net.Conn
}

func (s *tcpAppConnection) receive() (messages.Message, error) {
	buf := make([]byte, 64*1024)
	readBytes, readErr := s.conn.Read(buf)
	if readErr != nil {
		return messages.Message{}, fmt.Errorf("Failed to read from TCP connection %w", readErr)
	}
	return messages.NewFrame(s.theSessionID, buf[:readBytes]), nil
}

func (s *tcpAppConnection) write(m messages.Message) error {
	_, writeErr := s.conn.Write(messages.Body(m))
	if writeErr != nil {
		return fmt.Errorf("Failed to write to TCP connection %w", writeErr)
	}
	return writeErr
}

func (s *tcpAppConnection) sessionID() string {
	return s.theSessionID
}

type appConnection interface {
	receive() (messages.Message, error)
	write(m messages.Message) error
	sessionID() string
}
