package svcdetector

import (
	"sync"

	"github.com/glothriel/wormhole/pkg/peers"
)

type exposedServicesRegistry interface {
	all() []registryItem
	isExposed(app peers.App, svcParser serviceWrapper) bool
	markAsExposed(app peers.App, svcParser serviceWrapper)
	markAsWithdrawn(app peers.App, svcParser serviceWrapper)
}

type registryItem struct {
	apps    []peers.App
	service serviceWrapper
}

type defaultExposedServicesRegistry struct {
	registryMap map[string]registryItem
	mtx         *sync.Mutex
}

func (registry *defaultExposedServicesRegistry) all() []registryItem {
	registry.mtx.Lock()
	defer registry.mtx.Unlock()
	theList := []registryItem{}
	for _, registryItem := range registry.registryMap {
		theList = append(theList, registryItem)
	}
	return theList
}

func (registry *defaultExposedServicesRegistry) isExposed(app peers.App, service serviceWrapper) bool {
	registry.mtx.Lock()
	defer registry.mtx.Unlock()
	item, ok := registry.registryMap[service.id()]
	if !ok {
		return false
	}
	for _, exposedApp := range item.apps {
		if exposedApp.Name == app.Name && exposedApp.Address == app.Address {
			return true
		}
	}
	return false
}

func (registry *defaultExposedServicesRegistry) markAsExposed(app peers.App, service serviceWrapper) {
	registry.mtx.Lock()
	defer registry.mtx.Unlock()
	_, ok := registry.registryMap[service.id()]
	previousApps := []peers.App{}
	if ok {
		previousApps = registry.registryMap[service.id()].apps
	}
	registry.registryMap[service.id()] = registryItem{
		service: service,
		apps:    append(previousApps, app),
	}
}

func (registry *defaultExposedServicesRegistry) markAsWithdrawn(app peers.App, service serviceWrapper) {
	registry.mtx.Lock()
	defer registry.mtx.Unlock()
	item, ok := registry.registryMap[service.id()]
	if !ok {
		return
	}
	newApps := []peers.App{}
	for _, exposedApp := range item.apps {
		if exposedApp.Name == app.Name && exposedApp.Address == app.Address {
			continue
		}
		newApps = append(newApps, exposedApp)
	}
	registry.registryMap[service.id()] = registryItem{
		apps:    newApps,
		service: item.service,
	}
}

func newDefaultExposedServicesRegistry() exposedServicesRegistry {
	return &defaultExposedServicesRegistry{
		registryMap: make(map[string]registryItem),
		mtx:         &sync.Mutex{},
	}
}
