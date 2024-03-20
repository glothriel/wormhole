package client

import (
	"time"

	"github.com/avast/retry-go"
	"github.com/glothriel/wormhole/pkg/grtn"
	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
)

// Exposer exposes given apps via the peer
type Exposer struct {
	Peer peers.Peer
}

// Expose connects to the peer and instructs it to expose the apps
func (e *Exposer) Expose(appManager AppStateManager) error {
	appRegistry := newAppAddressRegistry()
	connectionRegistry := newAppConnectionRegistry(appRegistry)

	peerDisconnected := make(chan bool)
	grtn.Go(e.manageRegisteringAndUnregisteringOfApps(appManager, appRegistry, peerDisconnected))
	defer func() { peerDisconnected <- true }()

	grtn.Go(func() {
		for theMsg := range e.Peer.SessionEvents() {
			if messages.IsSessionClosed(theMsg) {
				connectionRegistry.delete(theMsg.SessionID)
			} else if messages.IsSessionOpened(theMsg) {
				theConnection, createErr := connectionRegistry.create(
					theMsg.SessionID,
					theMsg.AppName,
				)
				if createErr != nil {
					logrus.Errorf("Error when creating connection to app %s: %s", theMsg.AppName, createErr)
					continue
				}
				grtn.Go(e.forwardMessagesFromConnectionToPeer(theConnection))
			}
		}
	})

	for theMsg := range e.Peer.Frames() {
		if messages.IsPing(theMsg) {
			continue
		}
		var theConnection *appConnection
		if retryErr := retry.Do(func() error {
			var upstreamConnectionErr error
			theConnection, upstreamConnectionErr = connectionRegistry.get(
				theMsg.SessionID,
			)
			return upstreamConnectionErr
		}, retry.Attempts(20), retry.Delay(time.Millisecond*1)); retryErr != nil {
			logrus.Errorf(
				"Session ID `%s` does not have port opened - closing orchestrator", theMsg.SessionID,
			)
			connectionRegistry.delete(theMsg.SessionID)
			return retryErr
		}

		if messages.IsFrame(theMsg) {
			theConnection.outbox() <- theMsg
		}
		if messages.IsSessionClosed(theMsg) {
			connectionRegistry.delete(theMsg.SessionID)
		}
	}

	return nil
}

func (e *Exposer) manageRegisteringAndUnregisteringOfApps(
	appManager AppStateManager, appRegistry *appAddressRegistry, peerDisconnected chan bool,
) func() {
	return func() {
		changes := appManager.Changes()
		for {
			select {
			case change := <-changes:
				if change.State == AppStateChangeAdded {
					logrus.Infof("New app added: %s on %s", change.App.Name, change.App.Address)
					if sendErr := e.Peer.Send(messages.NewAppAdded(change.App.Name, change.App.Address)); sendErr != nil {
						logrus.Errorf("Could not send app added message to the peer: %v", sendErr)
					}
					appRegistry.register(change.App.Name, change.App.Address)
				} else if change.State == AppStateChangeWithdrawn {
					logrus.Infof("App withdrawn: %s", change.App.Name)
					if sendErr := e.Peer.Send(messages.NewAppWithdrawn(change.App.Name)); sendErr != nil {
						logrus.Errorf("Could not send app withdrawn message to the peer: %v", sendErr)
					}
					appRegistry.unregister(change.App.Name)
				} else {
					logrus.Errorf("Unknown app state change: %s", change.State)
				}
			case <-peerDisconnected:
				return
			}
		}
	}
}

func (e *Exposer) forwardMessagesFromConnectionToPeer(connection *appConnection) func() {
	return func() {
		defer func() {
			logrus.Debug("Stopped orchestrating TCP connection")
		}()
		for theMsg := range connection.inbox() {
			logrus.Debug("Received message over TCP")
			writeErr := e.Peer.Send(messages.WithAppName(theMsg, connection.appName))
			if writeErr != nil {
				logrus.Errorf("Could not send the message to peer: %v", writeErr)
			}
			logrus.Debug("Transimitted message to peer")
		}
	}
}

// NewExposer creates Exposer instances
func NewExposer(peer peers.Peer) *Exposer {
	return &Exposer{
		Peer: peer,
	}
}
