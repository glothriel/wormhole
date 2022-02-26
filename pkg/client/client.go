package client

import (
	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
)

type staticAppStateManager struct {
	Apps    []peers.App
	theChan chan AppStateChange
}

func (manager staticAppStateManager) Changes() chan AppStateChange {
	theChan := make(chan AppStateChange, len(manager.Apps))
	for _, app := range manager.Apps {
		theChan <- AppStateChange{
			App:   app,
			State: AppStateChangeAdded,
		}
	}
	return theChan
}

func NewStaticAppStateManager(apps []peers.App) AppStateManager {
	return &staticAppStateManager{Apps: apps}
}

type AppStateManager interface {
	Changes() chan AppStateChange
}

const (
	AppStateChangeAdded     = "added"
	AppStateChangeWithdrawn = "withdrawn"
)

type AppStateChange struct {
	App   peers.App
	State string
}

// Exposer exposes given apps via the peer
type Exposer struct {
	Peer peers.Peer
}

// Expose connects to the peer and instructs it to expose the apps
func (e *Exposer) Expose(appManager AppStateManager) error {
	appRegistry := newAppAddressRegistry()
	connectionRegistry := newAppConnectionRegistry(appRegistry)

	peerDisconnected := make(chan bool)
	go func() {
		keepLooping := true
		changes := appManager.Changes()
		for keepLooping {
			select {
			case change := <-changes:
				if change.State == AppStateChangeAdded {
					if sendErr := e.Peer.Send(messages.NewAppAdded(change.App.Name)); sendErr != nil {
						logrus.Errorf("Could not send app added message to the peer: %v", sendErr)
					}
					appRegistry.register(change.App.Name, change.App.Address)
				} else if change.State == AppStateChangeWithdrawn {
					if sendErr := e.Peer.Send(messages.NewAppWithdrawn(change.App.Name)); sendErr != nil {
						logrus.Errorf("Could not send app withdrawn message to the peer: %v", sendErr)
					}
					appRegistry.unregister(change.App.Name)
				} else {
					logrus.Errorf("Unknown app state change: %s", change.State)
				}
			case <-peerDisconnected:
				keepLooping = false
			}
		}
	}()
	defer close(peerDisconnected)

	for theMsg := range e.Peer.Frames() {
		if messages.IsPing(theMsg) {
			continue
		}
		var appConnection *appConnection
		appConnection, upstreamConnectionErr := connectionRegistry.get(
			theMsg.SessionID,
		)
		if upstreamConnectionErr != nil {
			var createErr error
			appConnection, createErr = connectionRegistry.create(
				theMsg.SessionID,
				theMsg.AppName,
			)
			if createErr != nil {
				logrus.Errorf("Error when creating connection to app %s: %s", theMsg.AppName, createErr)
			}
			go func() {
				defer func() {
					logrus.Debug("Stopped orchestrating TCP connection")
				}()
				for theMsg := range appConnection.inbox() {
					logrus.Debug("Received message over TCP")
					writeErr := e.Peer.Send(messages.WithAppName(theMsg, theMsg.AppName))
					if writeErr != nil {
						panic(writeErr)
					}
					logrus.Debug("Transimitted message to peer")
				}
			}()
		}
		if messages.IsFrame(theMsg) {
			appConnection.outbox() <- theMsg
		}
		if messages.IsDisconnect(theMsg) {
			connectionRegistry.delete(theMsg.SessionID)
		}
	}

	return nil
}

// NewExposer creates Exposer instances
func NewExposer(peer peers.Peer) *Exposer {
	return &Exposer{
		Peer: peer,
	}
}
