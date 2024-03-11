package client

import (
	"errors"
	"io"
	"net"
	"strings"

	"github.com/glothriel/wormhole/pkg/events"
	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/ps"
	"github.com/sirupsen/logrus"
)

type appConnection struct {
	sessionID string
	appName   string

	connection net.Conn

	theOutbox chan messages.Message
}

func (e *appConnection) outbox() chan messages.Message {
	return e.theOutbox
}

func (e *appConnection) terminate() {
	defer func() {
		if r := recover(); r != nil {
			logrus.Debugf("Recovered in %s", r)
		}
	}()
	if closeErr := e.connection.Close(); closeErr != nil {
		logrus.Errorf("Failed closing TCP connection: %v", closeErr)
	}
	close(e.theOutbox)
}

func newAppConnection(sessionID, address, appName string, bus ps.PubSub) (*appConnection, error) {
	logrus.Tracef("Dial %s", address)
	conn, dialErr := net.Dial("tcp", address)
	if dialErr != nil {
		return nil, dialErr
	}

	theConnection := &appConnection{
		sessionID:  sessionID,
		connection: conn,
		theOutbox:  make(chan messages.Message),

		appName: appName,
	}

	go func() {
		defer func() {
			logrus.Debug("Closing TCP connection outbox")
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
				logrus.Debugf("Recovered in %s", r)
			}
			logrus.Debug("Closing TCP connection inbox")
		}()
		for {
			buf := make([]byte, 1024*64)

			readBytes, err := theConnection.connection.Read(buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					theConnection.terminate()
				} else if !strings.Contains(err.Error(), "use of closed network connection") {
					logrus.Errorf("Failed to read TCP connection: %v", err)
				}
				return
			}

			msgBody := make([]byte, readBytes)
			for i := 0; i < readBytes; i++ {
				msgBody[i] = buf[i]
			}
			bus.Publish(events.SessionAppDataSentTopic(sessionID, appName), ps.NewContext(), messages.NewPacket(theConnection.sessionID, msgBody))

		}
	}()

	return theConnection, nil
}
