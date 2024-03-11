package client

import (
	"fmt"

	"github.com/glothriel/wormhole/pkg/events"
	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/ps"
)

func OnLocalAppExposed(pubSub ps.PubSub, cb func(ctx *ps.Context, v peers.App) error) {
	pubSub.Subscribe(events.LocalAppExposedTopic, func(ctx *ps.Context, v any) error {
		vAsApp, ok := v.(peers.App)
		if !ok {
			return fmt.Errorf("expected peers.App, got %T", v)
		}
		return cb(ctx, vAsApp)
	})
}

func OnLocalAppWithdrawn(pubSub ps.PubSub, cb func(ctx *ps.Context, v peers.App) error) {
	pubSub.Subscribe(events.LocalAppWithdrawnTopic, func(ctx *ps.Context, v any) error {
		vAsApp, ok := v.(peers.App)
		if !ok {
			return fmt.Errorf("expected peers.App, got %T", v)
		}
		return cb(ctx, vAsApp)
	})
}

func OnSessionStarted(pubSub ps.PubSub, cb func(ctx *ps.Context, sessionID, appName string) error) {
	pubSub.Subscribe(events.SessionStartedTopic(".*"), func(ctx *ps.Context, v any) error {
		vsAsMsg, ok := v.(messages.Message)
		if !ok {
			return fmt.Errorf("expected messages.Message, got %T", v)
		}
		return cb(ctx, vsAsMsg.SessionID, vsAsMsg.AppName)
	})
}

func OnSessionFinished(pubSub ps.PubSub, sessionID string, cb func(ctx *ps.Context, sessionID string) error) {
	pubSub.Subscribe(events.SessionFinishedTopic(".*"), func(ctx *ps.Context, v any) error {
		vsAsMsg, ok := v.(messages.Message)
		if !ok {
			return fmt.Errorf("expected messages.Message, got %T", v)
		}
		return cb(ctx, vsAsMsg.SessionID)
	}, sessionID)
}

func OnSessionClientData(pubSub ps.PubSub, sessionID string, appName string, cb func(ctx *ps.Context, msg messages.Message) error) {
	pubSub.Subscribe(events.SessionClientDataSentTopic(sessionID, appName), func(ctx *ps.Context, v any) error {
		vsAsMsg, ok := v.(messages.Message)
		if !ok {
			return fmt.Errorf("expected messages.Message, got %T", v)
		}
		return cb(ctx, vsAsMsg)
	}, sessionID)
}

func OnSessionAppData(pubSub ps.PubSub, sessionID string, appName string, cb func(ctx *ps.Context, msg messages.Message) error) {
	pubSub.Subscribe(events.SessionAppDataSentTopic(sessionID, appName), func(ctx *ps.Context, v any) error {
		vsAsMsg, ok := v.(messages.Message)
		if !ok {
			return fmt.Errorf("expected messages.Message, got %T", v)
		}
		return cb(ctx, vsAsMsg)
	}, sessionID)
}

func OnPing(pubSub ps.PubSub, cb func(ctx *ps.Context) error) {
	pubSub.Subscribe(events.PingTopic, func(ctx *ps.Context, v any) error {
		return cb(ctx)
	})
}
