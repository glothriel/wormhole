package server

import (
	"sync"

	"github.com/glothriel/wormhole/pkg/peers"
)

// exposedAppsRegistry allows to add, retrieve and list a list of exposed apps
// along with information about the peer the app was exposed from
type exposedAppsRegistry struct {
	storage sync.Map
}

func (registry *exposedAppsRegistry) get(peer peers.Peer, app peers.App) (portOpener, bool) {
	peerMap, exists := registry.storage.Load(peer.Name())
	if !exists {
		return nil, false
	}
	val, exists := peerMap.(*sync.Map).Load(app.Name)
	if !exists {
		return nil, false
	}
	return val.(storedExposer).portOpener, true
}

func (registry *exposedAppsRegistry) store(peer peers.Peer, app peers.App, portOpener portOpener) {
	var peerMap *sync.Map
	peerMapInterface, exists := registry.storage.Load(peer.Name())
	if exists {
		peerMap = peerMapInterface.(*sync.Map)
	} else {
		peerMap = &sync.Map{}
		registry.storage.Store(peer.Name(), peerMap)
	}
	peerMap.Store(app.Name, storedExposer{
		portOpener: portOpener,
		app:        app,
		peer:       peer,
	})
}

func (registry *exposedAppsRegistry) delete(peer peers.Peer, app peers.App) {
	var peerMap *sync.Map
	peerMapInterface, exists := registry.storage.Load(peer.Name())
	if exists {
		peerMap = peerMapInterface.(*sync.Map)
	} else {
		peerMap = &sync.Map{}
	}
	peerMap.Delete(app.Name)
}

func (registry *exposedAppsRegistry) deleteAll(peer peers.Peer) {
	registry.storage.Delete(peer.Name())
}

func (registry *exposedAppsRegistry) items() []storedExposer {
	items := []storedExposer{}
	registry.storage.Range(func(k, internalMap interface{}) bool {
		internalMap.(*sync.Map).Range(func(k, storedExposerEntry interface{}) bool {
			items = append(items, storedExposerEntry.(storedExposer))
			return true
		})
		return true
	})
	return items
}

func newExposedAppsRegistry() *exposedAppsRegistry {
	return &exposedAppsRegistry{storage: sync.Map{}}
}

type storedExposer struct {
	portOpener portOpener
	peer       peers.Peer
	app        peers.App
}
