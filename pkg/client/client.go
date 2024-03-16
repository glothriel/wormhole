package client

import (
	"context"
	"fmt"
	"io"

	"github.com/glothriel/wormhole/pkg/events"
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
	appNameToHostname := newAppAddressRegistry()

	OnSessionStarted(e.bus,
		func(ctx context.Context, sessionID, appName string) error {
			sessionStartedCtx := ctx

			destination, upstreamNameFound := appNameToHostname.get(appName)
			if !upstreamNameFound {
				return fmt.Errorf("could not find app with name %s", appName)
			}
			OnLocalSessionAppData(e.bus, sessionID, appName, func(ctx context.Context, msg messages.Message) error {
				return e.Peer.Send(messages.WithContext(messages.WithAppName(msg, appName), sessionStartedCtx))
			})
			theConnection, sessionErr := dialApp(sessionID, destination, appName, e.bus)
			if sessionErr != nil {
				return sessionErr
			}
			OnSessionClientData(e.bus, sessionID, appName, func(ctx context.Context, msg messages.Message) error {
				return theConnection.send(msg)
			})
			OnSessionFinished(e.bus, sessionID, func(ctx context.Context, sessionID string) error {
				theConnection.terminate()
				e.bus.Unsubscribe(sessionID)
				return nil
			})

			go func() {
				for {
					nextMsg, nextMsgErr := theConnection.read()
					if nextMsgErr != nil {
						if nextMsgErr != io.EOF {
							logrus.Errorf("Error reading from app connection: %v", nextMsgErr)
						} else {
							e.bus.Publish(events.LocalSessionAppEOFTopic(sessionID, appName), sessionStartedCtx, nextMsg)
						}
						break
					}
					e.bus.Publish(events.LocalSessionAppDataSentTopic(sessionID, appName), sessionStartedCtx, nextMsg)
				}
			}()

			return nil
		},
	)

	OnLocalAppExposed(e.bus, func(ctx context.Context, v peers.App) error {
		appNameToHostname.register(v.Name, v.Address)
		return e.Peer.Send(messages.NewAppAdded(v.Name, v.Address))
	})
	OnLocalAppWithdrawn(
		e.bus,
		func(ctx context.Context, v peers.App) error {
			appNameToHostname.unregister(v.Name)
			return e.Peer.Send(messages.NewAppWithdrawn(v.Name))
		},
	)

	OnPing(e.bus, func(ctx context.Context) error {
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
