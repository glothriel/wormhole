package client

import "sync"

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
