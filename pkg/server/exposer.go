package server

import (
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/ports"
)

// AppExposer is responsible for keeping track of which apps are registered and their endpoints exported
type AppExposer interface {
	Expose(peer peers.Peer, app peers.App, router messageRouter) error
	Unexpose(peer peers.Peer, app peers.App) error
	Apps() []ExposedApp
}

// ExposedApp represents an app exposed on the server along with the peer the app is exposed from
type ExposedApp struct {
	App  peers.App
	Peer peers.Peer
}
type defaultAppExposer struct {
	registry      *exposedAppsRegistry
	portAllocator ports.Allocator
}

func (exposer *defaultAppExposer) Expose(peer peers.Peer, app peers.App, router messageRouter) error {
	portOpener, portOpenerErr := newPerAppPortOpener(app.Name, exposer.portAllocator)
	if portOpenerErr != nil {
		return portOpenerErr
	}
	peer.WhenClosed(func() {
		portOpener.close()
	})
	exposer.registry.store(peer, app, portOpener)
	go func() {
		for connection := range portOpener.connections() {
			handler := newAppConnectionHandler(
				peer,
				app,
				connection,
			)
			go handler.Handle(router)
		}
		exposer.registry.delete(peer, app)
	}()
	return nil
}

func (exposer *defaultAppExposer) Unexpose(peer peers.Peer, app peers.App) error {
	portOpener, found := exposer.registry.get(peer, app)
	if !found {
		return nil
	}
	if closeErr := portOpener.close(); closeErr != nil {
		return closeErr
	}
	exposer.registry.delete(peer, app)
	return nil
}

func (exposer *defaultAppExposer) Apps() []ExposedApp {
	allApps := []ExposedApp{}
	for _, storedExposerEntry := range exposer.registry.items() {
		allApps = append(allApps, ExposedApp{
			App:  storedExposerEntry.app,
			Peer: storedExposerEntry.peer,
		})
	}
	return allApps
}

// NewDefaultAppExposer creates defaultAppExposer instances
func NewDefaultAppExposer(portAllocator ports.Allocator) AppExposer {
	return &defaultAppExposer{
		registry:      newExposedAppsRegistry(),
		portAllocator: portAllocator,
	}
}
