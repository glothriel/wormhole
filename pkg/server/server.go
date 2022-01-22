package server

import (
	"fmt"

	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/router"
	"github.com/sirupsen/logrus"
)

// Server accepts Peers and opens ports for all the apps connected peers expose
type Server struct {
	peerFactory   peers.PeerFactory
	portAllocator PortAllocator

	portExposers map[string]*perAppPortExposer
}

// Start launches the server
func (l *Server) Start() error {
	peersChan, peerErr := l.peerFactory.Peers()
	if peerErr != nil {
		return fmt.Errorf("Failed to initialize new Peer: %w", peerErr)
	}
	for peer := range peersChan {
		theApps, appsErr := peer.Apps()
		if appsErr != nil {
			logrus.Error(appsErr)
			continue
		}

		msgs, receiveErr := peer.Receive()
		if receiveErr != nil {
			return receiveErr
		}
		messageRouter := router.NewMessageRouter(msgs)

		for _, app := range theApps {
			portExposer, portExposerErr := newPerAppPortExposer(app.Name, l.portAllocator)
			if portExposerErr != nil {
				return portExposerErr
			}
			peer.WhenClosed(func() {
				portExposer.terminate()
			})
			l.portExposers[fmt.Sprintf("%s-%s", peer.Name(), app.Name)] = portExposer
			doneChan := make(chan bool)
			go func(upstream peers.App, peer peers.Peer, portExposer *perAppPortExposer, done chan bool) {
				connections, connectionErr := portExposer.connections()
				if connectionErr != nil {
					logrus.Error(connectionErr)
					return
				}
				for connection := range connections {
					handler := newSessionHandler(
						peer,
						connection,
						upstream.Name,
					)
					go handler.Handle(messageRouter)
				}
				delete(l.portExposers, fmt.Sprintf("%s-%s", peer.Name(), upstream.Name))
			}(app, peer, portExposer, doneChan)
		}
	}
	return nil
}

// NewServer creates Server instances
func NewServer(peerFactory peers.PeerFactory, portAllocator PortAllocator) *Server {
	listener := &Server{
		peerFactory:   peerFactory,
		portAllocator: portAllocator,

		portExposers: make(map[string]*perAppPortExposer),
	}
	return listener
}
