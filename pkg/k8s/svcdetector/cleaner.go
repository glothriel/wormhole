// Package svcdetector orchestrates kubernetes integration
package svcdetector

import (
	"github.com/glothriel/wormhole/pkg/peers"
)

type itemToDelete registryItem

type cleaner interface {
	clean(services []serviceWrapper, registry exposedServicesRegistry) ([]itemToDelete, error)
}

// Cleans up apps originating from services, that previously had exposing annotations, but no longer have
type modifiedAnnotationsCleaner struct{}

func (cleaner modifiedAnnotationsCleaner) clean(
	services []serviceWrapper, registry exposedServicesRegistry,
) ([]itemToDelete, error) {
	itemsToDelete := []itemToDelete{}

	for _, svc := range services {
		for _, app := range svc.apps() {
			if !svc.shouldBeExposed() && registry.isExposed(app, svc) {
				itemsToDelete = append(itemsToDelete, itemToDelete{
					apps:    []peers.App{app},
					service: svc,
				})
			}
		}
	}
	return itemsToDelete, nil
}

// Cleans up apps originating from services, that were removed
type removedServicesCleaner struct{}

func (cleaner removedServicesCleaner) clean(
	services []serviceWrapper,
	registry exposedServicesRegistry,
) ([]itemToDelete, error) {
	itemsToDelete := []itemToDelete{}
	for _, exposedItem := range registry.all() {
		serviceFound := false
		for _, svc := range services {
			if svc.id() != exposedItem.service.id() {
				continue
			}
			serviceFound = true
		}
		if !serviceFound {
			itemsToDelete = append(itemsToDelete, itemToDelete(exposedItem))
		}
	}
	return itemsToDelete, nil
}

// Cleans up apps originating from services, that have ports removed
type removedPortsCleaner struct{}

func (cleaner removedPortsCleaner) clean(
	services []serviceWrapper,
	registry exposedServicesRegistry,
) ([]itemToDelete, error) {
	itemsToDelete := []itemToDelete{}
	for _, exposedItem := range registry.all() {
		for _, svc := range services {
			if svc.id() != exposedItem.service.id() {
				continue
			}
			for _, exposedApp := range exposedItem.apps {
				exposedAppFound := false
				for _, parsedApp := range svc.apps() {
					if parsedApp.Name == exposedApp.Name && parsedApp.Address == exposedApp.Address {
						exposedAppFound = true
					}
				}
				if !exposedAppFound {
					itemsToDelete = append(itemsToDelete, itemToDelete{
						service: exposedItem.service,
						apps:    []peers.App{exposedApp},
					})
				}
			}
		}
	}
	return itemsToDelete, nil
}
