package client

import (
	"fmt"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/ps"
	"github.com/sirupsen/logrus"
)

// Exposer exposes given apps via the peer
type Exposer struct {
	Peer peers.Peer
	bus  ps.PubSub
}

// Expose connects to the peer and instructs it to expose the apps
func (e *Exposer) Expose(appManager AppStateManager) error {
	appRegistry := newAppAddressRegistry()

	OnSessionStarted(e.bus,
		func(ctx *ps.Context, sessionID, appName string) error {

			destination, upstreamNameFound := appRegistry.get(appName)
			if !upstreamNameFound {
				return fmt.Errorf("could not find app with name %s", appName)
			}
			theConnection, sessionErr := newAppConnection(sessionID, destination, appName, e.bus)
			if sessionErr != nil {
				return sessionErr
			}

			OnSessionAppData(e.bus, sessionID, theConnection.appName, func(ctx *ps.Context, msg messages.Message) error {
				return e.Peer.Send(messages.WithAppName(msg, theConnection.appName))
			})

			OnSessionClientData(e.bus, sessionID, theConnection.appName, func(ctx *ps.Context, msg messages.Message) error {
				theConnection.outbox() <- msg
				return nil
			})

			OnSessionFinished(e.bus, sessionID, func(ctx *ps.Context, sessionID string) error {
				theConnection.terminate()
				return nil
			})

			return nil
		},
	)

	OnSessionFinished(e.bus, ".*", func(ctx *ps.Context, sessionID string) error {
		e.bus.Unsubscribe(sessionID)
		return nil
	})

	OnLocalAppExposed(e.bus, func(ctx *ps.Context, v peers.App) error {
		appRegistry.register(v.Name, v.Address)
		return e.Peer.Send(messages.NewAppAdded(v.Name, v.Address))
	})
	OnLocalAppWithdrawn(
		e.bus,
		func(ctx *ps.Context, v peers.App) error {
			appRegistry.unregister(v.Name)
			return e.Peer.Send(messages.NewAppWithdrawn(v.Name))
		},
	)

	OnPing(e.bus, func(ctx *ps.Context) error {
		logrus.Trace("Received ping")
		return nil
	})
	appManager.Register(e.bus)
	return nil
}

// NewExposer creates Exposer instances
func NewExposer(peer peers.Peer, pubSub ps.PubSub) *Exposer {

	return &Exposer{
		Peer: peer,
		bus:  pubSub,
	}
}
