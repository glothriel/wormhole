package server

import (
	"fmt"
	"sync"

	"github.com/glothriel/wormhole/pkg/peers"
)

// exposedAppsRegistry allows to add, retrieve and list a list of exposed apps
// along with information about the peer the app was exposed from
type exposedAppsRegistry struct {
	storage sync.Map
}

func (registry *exposedAppsRegistry) get(peer peers.Peer, app peers.App) (*perAppPortOpener, bool) {
	val, exists := registry.storage.Load(registry.hash(peer, app))
	if !exists {
		return nil, false
	}
	return val.(storedExposer).portOpener, true
}

func (registry *exposedAppsRegistry) store(peer peers.Peer, app peers.App, portOpener *perAppPortOpener) {
	registry.storage.Store(registry.hash(peer, app), storedExposer{
		portOpener: portOpener,
		app:        app,
		peer:       peer,
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
	portOpener *perAppPortOpener
	peer       peers.Peer
	app        peers.App
}
