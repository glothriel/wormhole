package client

import (
	"errors"
	"io"
	"net"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
)

type appConnection struct {
	sessionID string
	appName   string

	connection net.Conn

	theInbox  chan messages.Message
	theOutbox chan messages.Message
}

func (e *appConnection) inbox() chan messages.Message {
	return e.theInbox
}

func (e *appConnection) outbox() chan messages.Message {
	return e.theOutbox
}

func (e *appConnection) terminate() {
	if closeErr := e.connection.Close(); closeErr != nil {
		logrus.Errorf("Failed closing TCP connection: %v", closeErr)
	}
	close(e.theInbox)
	close(e.theOutbox)
}

func newAppConnection(sessionID, address, appName string, peer peers.Peer) (*appConnection, error) {
	conn, dialErr := net.Dial("tcp", address)
	if dialErr != nil {
		return nil, dialErr
	}

	theConnection := &appConnection{
		sessionID:  sessionID,
		connection: conn,

		theInbox:  make(chan messages.Message),
		theOutbox: make(chan messages.Message),

		appName: appName,
	}

	logger := logrus.WithField("session_id", theConnection.sessionID).WithField("peer", peer.Name())

	go func() {
		defer func() {
			logger.Debug("Stopped orchestrating TCP connection")
		}()
		for theMsg := range theConnection.inbox() {
			logger.Debug("Received message over TCP")
			writeErr := peer.Send(messages.WithAppName(theMsg, appName))
			if writeErr != nil {
				panic(writeErr)
			}
			logger.Debug("Transimitted message to peer")
		}
	}()

	go func() {
		defer func() {
			logger.Debug("Closing TCP connection outbox")
		}()
		for msg := range theConnection.outbox() {
			_, writeErr := theConnection.connection.Write(messages.Body(msg))
			if writeErr != nil {
				panic(writeErr)
			}
		}
	}()

	go func() {
		defer func() {
			logger.Debug("Closing TCP connection inbox")
		}()
		for {
			buf := make([]byte, 1024*64)

			i, err := theConnection.connection.Read(buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					theConnection.terminate()
					return
				}
				logger.Errorf("Failed to read TCP connection: %v", err)
			}
			theConnection.inbox() <- messages.NewFrame(theConnection.sessionID, buf[:i])
		}
	}()

	return theConnection, nil
}
