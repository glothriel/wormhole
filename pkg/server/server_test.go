package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/glothriel/wormhole/pkg/grtn"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type mockAppExposer struct {
	registerLastCalledWith   registerAndUnregisterCommonArgs
	unregisterLastCalledWith registerAndUnregisterCommonArgs
}

func (exposer *mockAppExposer) Expose(peer peers.Peer, app peers.App, router messageRouter) error {
	exposer.registerLastCalledWith = registerAndUnregisterCommonArgs{
		peer: peer, app: app,
	}
	return nil
}

func (exposer *mockAppExposer) Unexpose(peer peers.Peer, app peers.App) error {
	exposer.unregisterLastCalledWith = registerAndUnregisterCommonArgs{
		peer: peer, app: app,
	}
	return nil
}

func (exposer *mockAppExposer) Terminate(peer peers.Peer) error { return nil }

func (exposer *mockAppExposer) Apps() []ExposedApp {
	allApps := []ExposedApp{}
	return allApps
}

type registerAndUnregisterCommonArgs struct {
	peer peers.Peer
	app  peers.App
}

func TestServer_Start(t *testing.T) {
	firstPeer := peers.NewMockPeer()
	incomingPeers := make(chan peers.Peer)
	appExposer := &mockAppExposer{}
	firstApp := peers.App{Name: "tibia", Address: "localhost:7171"}
	theServer := &Server{
		peerFactory: peers.NewMockPeerFactory(incomingPeers),
		appExposer:  appExposer,
	}
	grtn.Go(func() {
		if startErr := theServer.Start(); startErr != nil {
			logrus.Fatal(startErr)
		}
	})
	grtn.Go(func() { incomingPeers <- firstPeer })

	firstPeer.AppEventsPeer <- peers.AppEvent{
		Type: peers.EventAppAdded,
		App:  firstApp,
	}

	assert.Nil(t, retry.Do(func() error {
		if firstApp != appExposer.registerLastCalledWith.app {
			return fmt.Errorf("%v should equal %v", appExposer.registerLastCalledWith.app, firstApp)
		}
		if appExposer.registerLastCalledWith.peer != firstPeer {
			return fmt.Errorf("%v should equal %v", appExposer.registerLastCalledWith.peer, firstPeer)
		}
		return nil
	},
		retry.Attempts(5),
		retry.Delay(time.Millisecond),
	))

	firstPeer.AppEventsPeer <- peers.AppEvent{
		Type: peers.EventAppWithdrawn,
		App:  firstApp,
	}
	assert.Nil(t, retry.Do(func() error {
		if appExposer.unregisterLastCalledWith.app != firstApp {
			return fmt.Errorf("%v should equal %v", appExposer.unregisterLastCalledWith.app, firstApp)
		}
		if appExposer.unregisterLastCalledWith.peer != firstPeer {
			return fmt.Errorf("%v should equal %v", appExposer.unregisterLastCalledWith.peer, firstPeer)
		}
		return nil
	},
		retry.Attempts(5),
		retry.Delay(time.Millisecond),
	))
}
