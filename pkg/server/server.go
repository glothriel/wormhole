package server

import (
	"fmt"

	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/router"
	"github.com/sirupsen/logrus"
)

// AppExposer is responsible for keeping track of which apps are registered and their endpoints exported
type AppExposer interface {
	Register(peer peers.Peer, app peers.App, router messageRouter) error
	Unregister(peer peers.Peer, app peers.App) error
	Apps() []ExposedApp
}

// Server accepts Peers and opens ports for all the apps connected peers expose
type Server struct {
	peerFactory peers.PeerFactory
	appExposer  AppExposer
}

// Start launches the server
func (l *Server) Start() error {
	peersChan, peerErr := l.peerFactory.Peers()
	if peerErr != nil {
		return fmt.Errorf("Failed to initialize new Peer: %w", peerErr)
	}
	for peer := range peersChan {
		l.handlePeer(peer)
	}
	return nil
}

// Start launches the server
func (l *Server) handlePeer(peer peers.Peer) error {
	msgs, receiveErr := peer.Receive()
	if receiveErr != nil {
		return receiveErr
	}
	messageRouter := router.NewMessageRouter(msgs)
	go func(peer peers.Peer) {
		for appStatus := range peer.AppStatusChanges() {
			if appStatus.Name == peers.AppStatusAdded {
				if registerErr := l.appExposer.Register(peer, appStatus.App, messageRouter); registerErr != nil {
					logrus.Error(registerErr)
					return
				}
			} else if appStatus.Name == peers.AppStatusWithdrawn {
				if registerErr := l.appExposer.Unregister(peer, appStatus.App); registerErr != nil {
					logrus.Error(registerErr)
					return
				}
			}
		}
	}(peer)
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
