package server

import (
	"fmt"
	"sync"

	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
)

type defaultAppExposer struct {
	registry      *exposedAppsRegistry
	portAllocator PortAllocator
}

func (exposer *defaultAppExposer) Register(peer peers.Peer, app peers.App, router messageRouter) error {
	portListener, portExposerErr := newPerAppPortExposer(app.Name, exposer.portAllocator)
	if portExposerErr != nil {
		return portExposerErr
	}
	peer.WhenClosed(func() {
		portListener.terminate()
	})
	exposer.registry.store(peer, app, portListener)
	doneChan := make(chan bool)
	go func(upstream peers.App, peer peers.Peer, portExposer *perAppPortExposer, done chan bool) {
		connections, connectionErr := portExposer.connections()
		if connectionErr != nil {
			logrus.Error(connectionErr)
			return
		}
		for connection := range connections {
			handler := newSessionHandler(
				peer,
				connection,
				upstream.Name,
			)
			go handler.Handle(router)
		}
		exposer.registry.delete(peer, app)
	}(app, peer, portListener, doneChan)
	return nil
}

func (exposer *defaultAppExposer) Unregister(peer peers.Peer, app peers.App) error {
	listener, found := exposer.registry.get(peer, app)
	if !found {
		return nil
	}
	if terminateErr := listener.terminate(); terminateErr != nil {
		return terminateErr
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
func NewDefaultAppExposer(portAllocator PortAllocator) AppExposer {
	return &defaultAppExposer{
		registry:      newExposedAppsRegistry(),
		portAllocator: portAllocator,
	}
}

// ExposedApp represents an app exposed on the server along with the peer the app is exposed from
type ExposedApp struct {
	App  peers.App
	Peer peers.Peer
}

type exposedAppsRegistry struct {
	storage sync.Map
}

func (registry *exposedAppsRegistry) get(peer peers.Peer, app peers.App) (*perAppPortExposer, bool) {
	val, exists := registry.storage.Load(registry.hash(peer, app))
	if !exists {
		return nil, false
	}
	return val.(storedExposer).exposer, true
}

func (registry *exposedAppsRegistry) store(peer peers.Peer, app peers.App, portExposer *perAppPortExposer) {
	registry.storage.Store(registry.hash(peer, app), storedExposer{
		exposer: portExposer,
		app:     app,
		peer:    peer,
	})
}
func (registry *exposedAppsRegistry) delete(peer peers.Peer, app peers.App) {
	registry.storage.Delete(registry.hash(peer, app))
}

func (registry *exposedAppsRegistry) hash(peer peers.Peer, app peers.App) string {
	return fmt.Sprintf("%s-%s", peer.Name(), app.Name)
}

func (registry *exposedAppsRegistry) items() []storedExposer {
	items := []storedExposer{}
	registry.storage.Range(func(k, storedExposerEntry interface{}) bool {
		items = append(items, storedExposerEntry.(storedExposer))
		return true
	})
	return items
}

func newExposedAppsRegistry() *exposedAppsRegistry {
	return &exposedAppsRegistry{storage: sync.Map{}}
}

type storedExposer struct {
	exposer *perAppPortExposer
	peer    peers.Peer
	app     peers.App
}
