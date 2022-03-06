package client

import (
	"sync"

	"github.com/glothriel/wormhole/pkg/peers"
)

// AppStateManager notifies the client about the state of exposed apps
type AppStateManager interface {
	Changes() chan AppStateChange
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

func (manager staticAppStateManager) Changes() chan AppStateChange {
	theChan := make(chan AppStateChange, len(manager.Apps))
	for _, app := range manager.Apps {
		theChan <- AppStateChange{
			App:   app,
			State: AppStateChangeAdded,
		}
	}
	return theChan
}

// NewStaticAppStateManager creates new AppStateManager for a static list of supported apps
func NewStaticAppStateManager(apps []peers.App) AppStateManager {
	return &staticAppStateManager{Apps: apps}
}

const (
	// AppStateChangeAdded is emmited when new app is exposed
	AppStateChangeAdded = "added"
	// AppStateChangeWithdrawn is emmited when app is withdrawn
	AppStateChangeWithdrawn = "withdrawn"
)

// AppStateChange is emmited when app state changes
type AppStateChange struct {
	App   peers.App
	State string
}
