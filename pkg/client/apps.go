package client

import (
	"context"
	"sync"

	"github.com/glothriel/wormhole/pkg/events"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/ps"
)

// AppStateManager notifies the client about the state of exposed apps
type AppStateManager interface {
	Register(ps.PubSub)
}

type appAddressRegistry struct {
	addresses sync.Map
}

func (registry *appAddressRegistry) get(appName string) (string, bool) {
	address, found := registry.addresses.Load(appName)
	if !found {
		return "", false
	}
	return address.(string), true
}

func (registry *appAddressRegistry) register(appName, address string) {
	registry.addresses.Store(appName, address)
}

func (registry *appAddressRegistry) unregister(appName string) {
	registry.addresses.Delete(appName)
}

func newAppAddressRegistry() *appAddressRegistry {
	return &appAddressRegistry{
		addresses: sync.Map{},
	}
}

type staticAppStateManager struct {
	Apps []peers.App
}

func (manager staticAppStateManager) Register(bus ps.PubSub) {
	for _, app := range manager.Apps {
		bus.Publish(
			events.LocalAppExposedTopic, context.Background(), app,
		)
	}
}

// NewStaticAppStateManager creates new AppStateManager for a static list of supported apps
func NewStaticAppStateManager(apps []peers.App) AppStateManager {
	return &staticAppStateManager{Apps: apps}
}
