package client

import (
	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
)

type staticAppStateManager struct {
	Apps []peers.App
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

// NewStaticAppStateManager creates new AppStateManager for a static list of supported apps
func NewStaticAppStateManager(apps []peers.App) AppStateManager {
	return &staticAppStateManager{Apps: apps}
}

// AppStateManager notifies the client about the state of exposed apps
type AppStateManager interface {
	Changes() chan AppStateChange
}

const (
	// AppStateChangeAdded is emmited when new app is exposed
	AppStateChangeAdded = "added"
	// AppStateChangeWithdrawn is emmited when app is withdrawn
	AppStateChangeWithdrawn = "withdrawn"
)

// AppStateChange is emmited when app state changes
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
	go e.manageRegisteringAndUnregisteringOfApps(appManager, appRegistry, peerDisconnected)
	defer func() { peerDisconnected <- true }()

	for theMsg := range e.Peer.Frames() {
		if messages.IsPing(theMsg) {
			continue
		}
		var theConnection *appConnection
		theConnection, upstreamConnectionErr := connectionRegistry.get(
			theMsg.SessionID,
		)
		if upstreamConnectionErr != nil {
			var createErr error
			theConnection, createErr = connectionRegistry.create(
				theMsg.SessionID,
				theMsg.AppName,
			)
			if createErr != nil {
				logrus.Errorf("Error when creating connection to app %s: %s", theMsg.AppName, createErr)
			}
			go e.forwardMessagesFromConnectionToPeer(theConnection)
		}
		if messages.IsFrame(theMsg) {
			theConnection.outbox() <- theMsg
		}
		if messages.IsDisconnect(theMsg) {
			connectionRegistry.delete(theMsg.SessionID)
		}
	}

	return nil
}

func (e *Exposer) manageRegisteringAndUnregisteringOfApps(
	appManager AppStateManager, appRegistry *appAddressRegistry, peerDisconnected chan bool,
) {
	keepLooping := true
	changes := appManager.Changes()
	for keepLooping {
		select {
		case change := <-changes:
			if change.State == AppStateChangeAdded {
				if sendErr := e.Peer.Send(messages.NewAppAdded(change.App.Name, change.App.Address)); sendErr != nil {
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
}

func (e *Exposer) forwardMessagesFromConnectionToPeer(connection *appConnection) {
	defer func() {
		logrus.Debug("Stopped orchestrating TCP connection")
	}()
	for theMsg := range connection.inbox() {
		logrus.Debug("Received message over TCP")
		writeErr := e.Peer.Send(messages.WithAppName(theMsg, theMsg.AppName))
		if writeErr != nil {
			panic(writeErr)
		}
		logrus.Debug("Transimitted message to peer")
	}
}

// NewExposer creates Exposer instances
func NewExposer(peer peers.Peer) *Exposer {
	return &Exposer{
		Peer: peer,
	}
}
