package server

import (
	"context"
	"fmt"

	"github.com/glothriel/wormhole/pkg/events"
	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/ps"
)

func OnRemoteAppExposed(pubSub ps.PubSub, peer peers.Peer, cb func(ctx context.Context, app peers.App) error) {
	pubSub.Subscribe(events.RemoteAppExposedTopic(peer.Name()), func(ctx context.Context, v any) error {
		vsAsApp, ok := v.(peers.App)
		if !ok {
			return fmt.Errorf("expected peers.App, got %T", v)
		}
		return cb(ctx, vsAsApp)
	})
}

func OnRemoteAppWithdrawn(pubSub ps.PubSub, peer peers.Peer, cb func(ctx context.Context, app peers.App) error) {
	pubSub.Subscribe(events.RemoteAppWithdrawnTopic(peer.Name()), func(ctx context.Context, v any) error {
		vsAsApp, ok := v.(peers.App)
		if !ok {
			return fmt.Errorf("expected peers.App, got %T", v)
		}
		return cb(ctx, vsAsApp)
	})
}

func OnSessionAppData(pubSub ps.PubSub, sessionID string, appName string, cb func(ctx context.Context, msg messages.Message) error) {
	pubSub.Subscribe(events.RemoteSessionAppDataSentTopic(sessionID, appName), func(ctx context.Context, v any) error {
		vsAsMsg, ok := v.(messages.Message)
		if !ok {
			return fmt.Errorf("expected messages.Message, got %T", v)
		}
		return cb(ctx, vsAsMsg)
	}, sessionID)
}

func OnLocalSessionStarted(pubSub ps.PubSub, sessionId, appName, peerName string, cb func(ctx context.Context, connection appConnection) error) {
	pubSub.Subscribe(events.LocalSessionStartedTopic(sessionId, appName, peerName), func(ctx context.Context, v any) error {
		vsAsAppConnection, ok := v.(appConnection)
		if !ok {
			return fmt.Errorf("expected server.appConnection, got %T", v)
		}
		return cb(ctx, vsAsAppConnection)
	})
}
