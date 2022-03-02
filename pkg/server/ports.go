package server

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/avast/retry-go"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/ports"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// perAppPortOpener exposes a port for every app and allows retrieving new connections for every app
type perAppPortOpener struct {
	appName  string
	listener net.Listener
	port     int
}

func (sm *perAppPortOpener) connections() chan appConnection {
	theChan := make(chan appConnection)
	go func(theChan chan appConnection) {
		defer func() { close(theChan) }()
		for {
			tcpC, acceptErr := sm.listener.Accept()
			if acceptErr != nil {
				if !errors.Is(acceptErr, net.ErrClosed) {
					logrus.Errorf("Failed to accept new TCP connection: %s", acceptErr)
				}
				return
			}
			sessionID := uuid.New().String()[:6]
			logrus.Infof(
				"New session ID %s", sessionID,
			)
			theSession := &tcpAppConnection{
				conn:         tcpC,
				theSessionID: sessionID,
			}
			theChan <- theSession
		}
	}(theChan)

	return theChan
}

func (sm *perAppPortOpener) listenAddr() string {
	return fmt.Sprintf("0.0.0.0:%d", sm.port)
}

func (sm *perAppPortOpener) close() error {
	return sm.listener.Close()
}

func newPerAppPortOpener(name string, allocator ports.Allocator) (*perAppPortOpener, error) {
	var listener net.Listener
	var freePort int
	if retryErr := retry.Do(
		func() error {
			var portErr error
			freePort, portErr = allocator.GetFreePort()
			if portErr != nil {
				return portErr
			}
			address := fmt.Sprintf("0.0.0.0:%d", freePort)
			var listenErr error
			listener, listenErr = net.Listen("tcp", address)
			return listenErr
		},
		// The ports can be "allocated" just by selecting random number from a range, so we should retry enough times
		// to be sure, that it works
		retry.Attempts(50),
		retry.Delay(time.Millisecond),
	); retryErr != nil {
		return nil, fmt.Errorf("Could not obtain a free port and start listening: %w", retryErr)
	}

	return &perAppPortOpener{
		appName:  name,
		listener: listener,
		port:     freePort,
	}, nil
}

type perAppPortOpenerFactory struct {
	portAllocator ports.Allocator
}

func (factory *perAppPortOpenerFactory) Create(app peers.App, peer peers.Peer) (portOpener, error) {
	theOpener, openerErr := newPerAppPortOpener(app.Name, factory.portAllocator)
	return theOpener, openerErr
}

// NewPerAppPortOpenerFactory implements PortOpenerFactory for standard opened TCP connection
func NewPerAppPortOpenerFactory(allocator ports.Allocator) PortOpenerFactory {
	return &perAppPortOpenerFactory{portAllocator: allocator}
}
