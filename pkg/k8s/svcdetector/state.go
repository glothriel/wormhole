package svcdetector

import (
	"time"

	"github.com/glothriel/wormhole/pkg/client"
	"github.com/glothriel/wormhole/pkg/grtn"
	"github.com/sirupsen/logrus"
)

type stateManager struct {
	repository        ServiceRepository
	notifier          *exposedServicesNotifier
	errorWaitInterval time.Duration
	registry          exposedServicesRegistry
	stateChangeChan   chan client.AppStateChange
}

func (manager *stateManager) Changes() chan client.AppStateChange {
	grtn.Go(func() {
		for {
			select {
			case createdService := <-manager.notifier.modifiedServices():
				if createdService.shouldBeExposed() {
					for _, app := range createdService.apps() {
						if !manager.registry.isExposed(app, createdService) {
							manager.stateChangeChan <- client.AppStateChange{
								App:   app,
								State: client.AppStateChangeAdded,
							}
							manager.registry.markAsExposed(app, createdService)
						}
					}
				}
			case <-manager.notifier.deletedServices():
				manager.cleanupRemoved()
			}
		}
	})

	return manager.stateChangeChan
}

func (manager *stateManager) cleanupRemoved() {
	cleaners := []cleaner{
		removedServicesCleaner{},
		removedPortsCleaner{},
		modifiedAnnotationsCleaner{},
	}
	itemsToDelete := []itemToDelete{}

	services, listErr := manager.repository.list()
	if listErr != nil {
		logrus.Errorf("Unable to cleanup exposed services: %v", listErr)
		return
	}
	for _, cleaner := range cleaners {
		itemsFromCleaner, cleanErr := cleaner.clean(services, manager.registry)
		if cleanErr != nil {
			logrus.Errorf("Unable to cleanup exposed services: %v", cleanErr)
			return
		}
		itemsToDelete = append(itemsFromCleaner, itemsFromCleaner...)
	}
	for _, itemToDelete := range itemsToDelete {
		for _, app := range itemToDelete.apps {
			manager.registry.markAsWithdrawn(app, itemToDelete.service)
			manager.stateChangeChan <- client.AppStateChange{
				App:   app,
				State: client.AppStateChangeWithdrawn,
			}
		}
	}
}

// NewK8sAppStateManager create AppStateManager instances, that expose kubernetes services
// (or not, judging on their annotations)
func NewK8sAppStateManager(
	svcRepository ServiceRepository,
	cleanupInterval time.Duration,
) client.AppStateManager {
	theManager := &stateManager{
		repository:        svcRepository,
		notifier:          newExposedServicesNotifier(svcRepository),
		errorWaitInterval: time.Second * 30,
		stateChangeChan:   make(chan client.AppStateChange),
		registry:          newDefaultExposedServicesRegistry(),
	}
	ticker := time.NewTicker(cleanupInterval)
	quit := make(chan struct{})
	grtn.Go(func() {
		for {
			select {
			case <-ticker.C:
				theManager.cleanupRemoved()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	})
	return theManager
}
