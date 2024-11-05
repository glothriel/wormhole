package svcdetector

import (
	"time"

	"github.com/glothriel/wormhole/pkg/apps"
	"github.com/sirupsen/logrus"
)

// AppStateManager is an interface for managing the state of apps
type AppStateManager interface {
	Changes() chan AppStateChange
}

// AppStateChange is a struct that represents a change in the app state
type AppStateChange struct {
	App   apps.App
	State string
}

const (
	// AppStateChangeAdded represents an app being added
	AppStateChangeAdded string = "added"
	// AppStateChangeWithdrawn represents an app being withdrawn
	AppStateChangeWithdrawn string = "withdrawn"
)

type stateManager struct {
	repository        ServiceRepository
	notifier          *exposedServicesNotifier
	errorWaitInterval time.Duration
	registry          exposedServicesRegistry
	stateChangeChan   chan AppStateChange
}

func (manager *stateManager) Changes() chan AppStateChange {
	go func() {
		for {
			select {
			case createdService := <-manager.notifier.modifiedServices():
				if createdService.shouldBeExposed() {
					for _, app := range createdService.apps() {
						if !manager.registry.isExposed(app, createdService) {
							manager.stateChangeChan <- AppStateChange{
								App:   app,
								State: AppStateChangeAdded,
							}
							manager.registry.markAsExposed(app, createdService)
						}
					}
				}
			case <-manager.notifier.deletedServices():
				manager.cleanupRemoved()
			}
		}
	}()

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
		itemsToDelete = append(itemsToDelete, itemsFromCleaner...)
	}

	for _, itemToDelete := range itemsToDelete {
		for _, app := range itemToDelete.apps {
			manager.registry.markAsWithdrawn(app, itemToDelete.service)
			manager.stateChangeChan <- AppStateChange{
				App:   app,
				State: AppStateChangeWithdrawn,
			}
		}
	}
}

// NewK8sAppStateManager create AppStateManager instances, that expose kubernetes services
// (or not, judging on their annotations)
func NewK8sAppStateManager(
	svcRepository ServiceRepository,
	cleanupInterval time.Duration,
) AppStateManager {
	theManager := &stateManager{
		repository:        svcRepository,
		notifier:          newExposedServicesNotifier(svcRepository),
		errorWaitInterval: time.Second * 30,
		stateChangeChan:   make(chan AppStateChange),
		registry:          newDefaultExposedServicesRegistry(),
	}
	ticker := time.NewTicker(cleanupInterval)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				theManager.cleanupRemoved()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
	return theManager
}
