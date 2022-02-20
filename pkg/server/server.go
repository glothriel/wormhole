package server

import (
	"fmt"

	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/router"
	"github.com/sirupsen/logrus"
)

// Server accepts Peers and opens ports for all the apps connected peers expose
type Server struct {
	peerFactory peers.PeerFactory
	appExposer  AppExposer
}

// Start launches the server
func (l *Server) Start() error {
	peersChan, peerErr := l.peerFactory.Peers()
	if peerErr != nil {
		return fmt.Errorf("Failed to start Peer factory %w", peerErr)
	}
	for peer := range peersChan {
		msgs, receiveErr := peer.Receive()
		if receiveErr != nil {
			return receiveErr
		}
		messageRouter := router.NewMessageRouter(msgs)
		go func(peer peers.Peer) {
			for appEvent := range peer.AppEvents() {
				if appEvent.Type == peers.EventAppAdded {
					if registerErr := l.appExposer.Expose(peer, appEvent.App, messageRouter); registerErr != nil {
						logrus.Error(registerErr)
						return
					}
				} else if appEvent.Type == peers.EventAppWithdrawn {
					if unregisterErr := l.appExposer.Unexpose(peer, appEvent.App); unregisterErr != nil {
						logrus.Error(unregisterErr)
						return
					}
				}
			}
			if terminateErr := l.appExposer.Terminate(peer); terminateErr != nil {
				logrus.Warnf("could not terminate peer gracefully: %v", terminateErr)
			}
		}(peer)
	}
	return nil
}

// NewServer creates Server instances
func NewServer(peerFactory peers.PeerFactory, appExposer AppExposer) *Server {
	listener := &Server{
		peerFactory: peerFactory,
		appExposer:  appExposer,
	}
	return listener
}
