package client

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/ps"
	"github.com/sirupsen/logrus"
)

type appConnection struct {
	sessionID string
	appName   string

	connection net.Conn
}

func (e *appConnection) terminate() {
	defer func() {
		if r := recover(); r != nil {
			if !strings.Contains(fmt.Sprintf("%s", r), "close of closed channel") {
				logrus.Errorf("Recovered in %s", r)
			}
		}
	}()
	if closeErr := e.connection.Close(); closeErr != nil {
		if !strings.Contains(closeErr.Error(), "use of closed network connection") {
			logrus.Errorf("Failed closing TCP connection: %v", closeErr)
		}
	}
}

func (e *appConnection) send(msg messages.Message) error {
	theBody := messages.Body(msg)
	_, writeErr := e.connection.Write(theBody)
	if writeErr != nil {
		return writeErr
	}
	return nil
}

func (e *appConnection) read() (messages.Message, error) {
	buf := make([]byte, 1024*64)

	readBytes, err := e.connection.Read(buf)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return messages.Message{}, err
		} else if !strings.Contains(err.Error(), "use of closed network connection") {
			return messages.Message{}, fmt.Errorf("Failed to read TCP connection: %v", err)
		}
		return messages.Message{}, io.EOF
	}

	msgBody := make([]byte, readBytes)
	for i := 0; i < readBytes; i++ {
		msgBody[i] = buf[i]
	}
	return messages.NewPacketFromApp(e.sessionID, msgBody), nil
}

func dialApp(sessionID, address, appName string, bus ps.PubSub) (*appConnection, error) {
	logrus.Tracef("Dial %s", address)
	conn, dialErr := net.Dial("tcp", address)
	if dialErr != nil {
		return nil, dialErr
	}

	theConnection := &appConnection{
		sessionID:  sessionID,
		connection: conn,

		appName: appName,
	}

	return theConnection, nil
}
