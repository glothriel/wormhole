package server

import (
	"fmt"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/router"
	"github.com/sirupsen/logrus"
)

// Server accepts Peers and opens ports for all the apps connected peers expose
type Server struct {
	peerFactory peers.PeerFactory
	appExposer  AppExposer
}

func (server *Server) onAppAdded(peer peers.Peer, event peers.AppEvent) error {
	exposedApp, registerErr := server.appExposer.Expose(
		peer, event.App, router.NewPacketRouter(peer.Packets()),
	)
	if registerErr != nil {
		return registerErr
	}
	return peer.Send(
		messages.NewAppConfirmed(exposedApp.App.Name, exposedApp.App.Address),
	)
}
func (server *Server) onAppWithdrawn(peer peers.Peer, event peers.AppEvent) error {
	return server.appExposer.Unexpose(peer, event.App)
}

// Start launches the server
func (server *Server) Start() error {
	peersChan, peerErr := server.peerFactory.Peers()
	if peerErr != nil {
		return fmt.Errorf("Failed to start Peer factory %w", peerErr)
	}
	for peer := range peersChan {
		go func(peer peers.Peer) {
			logrus.Infof("Peer `%s` connected", peer.Name())
			for appEvent := range peer.AppEvents() {
				handler, ok := map[string]func(peers.Peer, peers.AppEvent) error{
					peers.EventAppAdded:     server.onAppAdded,
					peers.EventAppWithdrawn: server.onAppWithdrawn,
				}[appEvent.Type]
				if ok {
					if handlerErr := handler(peer, appEvent); handlerErr != nil {
						logrus.Error(handlerErr)
						return
					}
				}
			}
			if terminateErr := server.appExposer.Terminate(peer); terminateErr != nil {
				logrus.Warnf("could not terminate peer `%s` gracefully: %v", peer.Name(), terminateErr)
			} else {
				logrus.Infof("Peer `%s` disconnected", peer.Name())
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
