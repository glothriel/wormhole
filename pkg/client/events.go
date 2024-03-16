package client

import (
	"context"
	"fmt"

	"github.com/glothriel/wormhole/pkg/events"
	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/ps"
)

func OnLocalAppExposed(pubSub ps.PubSub, cb func(ctx context.Context, v peers.App) error) {
	pubSub.Subscribe(events.LocalAppExposedTopic, func(ctx context.Context, v any) error {
		vAsApp, ok := v.(peers.App)
		if !ok {
			return fmt.Errorf("expected peers.App, got %T", v)
		}
		return cb(ctx, vAsApp)
	})
}

func OnLocalAppWithdrawn(pubSub ps.PubSub, cb func(ctx context.Context, v peers.App) error) {
	pubSub.Subscribe(events.LocalAppWithdrawnTopic, func(ctx context.Context, v any) error {
		vAsApp, ok := v.(peers.App)
		if !ok {
			return fmt.Errorf("expected peers.App, got %T", v)
		}
		return cb(ctx, vAsApp)
	})
}

func OnSessionStarted(pubSub ps.PubSub, cb func(ctx context.Context, sessionID, appName string) error) {
	pubSub.Subscribe(events.RemoteSessionStartedTopic(".*"), func(ctx context.Context, v any) error {
		vsAsMsg, ok := v.(messages.Message)
		if !ok {
			return fmt.Errorf("expected messages.Message, got %T", v)
		}
		return cb(ctx, vsAsMsg.SessionID, vsAsMsg.AppName)
	})
}

func OnSessionFinished(pubSub ps.PubSub, sessionID string, cb func(ctx context.Context, sessionID string) error) {
	pubSub.Subscribe(events.RemoteSessionFinishedTopic(sessionID), func(ctx context.Context, v any) error {
		vsAsMsg, ok := v.(messages.Message)
		if !ok {
			return fmt.Errorf("expected messages.Message, got %T", v)
		}
		return cb(ctx, vsAsMsg.SessionID)
	}, sessionID)
}

func OnSessionClientData(pubSub ps.PubSub, sessionID string, appName string, cb func(ctx context.Context, msg messages.Message) error) {
	pubSub.Subscribe(events.RemoteSessionClientDataSentTopic(sessionID, appName), func(ctx context.Context, v any) error {
		vsAsMsg, ok := v.(messages.Message)
		if !ok {
			return fmt.Errorf("expected messages.Message, got %T", v)
		}
		return cb(ctx, vsAsMsg)
	}, sessionID)
}

func OnLocalSessionAppData(pubSub ps.PubSub, sessionID string, appName string, cb func(ctx context.Context, msg messages.Message) error) {
	pubSub.Subscribe(events.LocalSessionAppDataSentTopic(sessionID, appName), func(ctx context.Context, v any) error {
		vsAsMsg, ok := v.(messages.Message)
		if !ok {
			return fmt.Errorf("expected messages.Message, got %T", v)
		}
		return cb(ctx, vsAsMsg)
	}, sessionID)
}

func OnSessionAppEOF(pubSub ps.PubSub, sessionID string, appName string, cb func(ctx context.Context) error) {
	pubSub.Subscribe(events.LocalSessionAppEOFTopic(sessionID, appName), func(ctx context.Context, v any) error {
		return cb(ctx)
	}, sessionID)
}

func OnPing(pubSub ps.PubSub, cb func(ctx context.Context) error) {
	pubSub.Subscribe(events.PingTopic, func(ctx context.Context, v any) error {
		return cb(ctx)
	})
}
