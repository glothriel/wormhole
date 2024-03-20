package server

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/glothriel/wormhole/pkg/grtn"
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

func (handler *appConnectionHandler) handleIncomingPeerMessages(router messageRouter) func() {
	return func() {
		for message := range router.Get(handler.appConnection.sessionID()) {
			if messages.IsFrame(message) {
				if writeErr := handler.appConnection.write(message); writeErr != nil {
					logrus.Fatal(writeErr)
				}
			}
		}
	}
}

func (handler *appConnectionHandler) handleIncomingAppMessages(router messageRouter) func() {
	return func() {
		defer router.Done(handler.appConnection.sessionID())
		for {
			downstreamMsg, receiveErr := handler.appConnection.receive()
			if receiveErr != nil {
				if errors.Is(receiveErr, io.EOF) {
					if sessionClosedErr := handler.peer.Send(
						messages.NewSessionClosed(handler.appConnection.sessionID(), handler.app.Name),
					); sessionClosedErr != nil {
						logrus.Errorf(
							"Failed to notify peer about closed session: %v", sessionClosedErr,
						)
					}
				} else {
					logrus.Error(receiveErr)
				}
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
}

func (handler *appConnectionHandler) Handle(router messageRouter) func() {
	return func() {

		if sendErr := handler.peer.Send(messages.NewSessionOpened(
			handler.appConnection.sessionID(),
			handler.app.Name,
		)); sendErr != nil {
			logrus.Errorf("Could not notify the peer about new opened session")
		}
		grtn.Go(handler.handleIncomingPeerMessages(router))
		grtn.Go(handler.handleIncomingAppMessages(router))
	}
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
	msgBody := make([]byte, readBytes)
	for i := 0; i < readBytes; i++ {
		msgBody[i] = buf[i]
	}
	return messages.NewFrame(s.theSessionID, msgBody), nil
}

func (s *tcpAppConnection) write(m messages.Message) error {
	theBody := messages.Body(m)
	_, writeErr := s.conn.Write(theBody)
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
