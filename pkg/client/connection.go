package client

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/glothriel/wormhole/pkg/messages"
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

func newAppConnection(sessionID, address, appName string) (*appConnection, error) {
	logrus.Debugf("Dial %s", address)
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

	logger := logrus.WithField("session_id", theConnection.sessionID)

	go func() {
		defer func() {
			logger.Debug("Closing TCP connection outbox")
		}()
		for msg := range theConnection.outbox() {
			theBody := messages.Body(msg)
			_, writeErr := theConnection.connection.Write(theBody)
			if writeErr != nil {
				logrus.Debugf("Failed writing message: %s", msg.Type)
			}
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Debugf("Recovered in %s", r)
			}
			logger.Debug("Closing TCP connection inbox")
		}()
		for {
			buf := make([]byte, 1024*64)

			readBytes, err := theConnection.connection.Read(buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					theConnection.terminate()
				} else if !strings.Contains(err.Error(), "use of closed network connection") {
					logger.Errorf("Failed to read TCP connection: %v", err)
				}
				return
			}

			msgBody := make([]byte, readBytes)
			for i := 0; i < readBytes; i++ {
				msgBody[i] = buf[i]
			}

			theConnection.inbox() <- messages.NewFrame(theConnection.sessionID, msgBody)
		}
	}()

	return theConnection, nil
}

type appConnectionsRegistry struct {
	upstreamConnections sync.Map
	addresses           *appAddressRegistry
}

func (registry *appConnectionsRegistry) create(
	sessionID, appName string,
) (*appConnection, error) {
	session, found := registry.upstreamConnections.Load(sessionID)
	if !found {
		destination, upstreamNameFound := registry.addresses.get(appName)
		if !upstreamNameFound {
			return nil, fmt.Errorf("Could not find app with name %s", appName)
		}
		logrus.WithField("session_id", sessionID).Infof("Creating new client session on %s", destination)
		theSession, sessionErr := newAppConnection(sessionID, destination, appName)
		if sessionErr != nil {
			return nil, sessionErr
		}
		registry.upstreamConnections.Store(sessionID, theSession)
		return theSession, nil
	}
	return session.(*appConnection), nil
}

func (registry *appConnectionsRegistry) get(
	sessionID string,
) (*appConnection, error) {
	session, found := registry.upstreamConnections.Load(sessionID)
	if !found {
		return nil, fmt.Errorf("Could not find connection with ID %s", sessionID)
	}
	return session.(*appConnection), nil
}

func (registry *appConnectionsRegistry) delete(sessionID string) {
	session, found := registry.upstreamConnections.Load(sessionID)
	if found {
		session.(*appConnection).terminate()
		registry.upstreamConnections.Delete(sessionID)
	}
}

func newAppConnectionRegistry(addresses *appAddressRegistry) *appConnectionsRegistry {
	return &appConnectionsRegistry{
		upstreamConnections: sync.Map{},
		addresses:           addresses,
	}
}
