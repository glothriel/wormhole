package svcdetector

import "github.com/glothriel/wormhole/pkg/grtn"

type exposedServicesNotifier struct {
	createUpdateChan chan serviceWrapper
	deleteChan       chan serviceWrapper
}

func (notifier *exposedServicesNotifier) modifiedServices() chan serviceWrapper {
	return notifier.createUpdateChan
}

func (notifier *exposedServicesNotifier) deletedServices() chan serviceWrapper {
	return notifier.deleteChan
}

func newExposedServicesNotifier(repository ServiceRepository) *exposedServicesNotifier {
	theNotifier := &exposedServicesNotifier{
		createUpdateChan: make(chan serviceWrapper),
		deleteChan:       make(chan serviceWrapper),
	}
	grtn.Go(func() {
		for event := range repository.watch() {
			if event.isAddedOrModified() {
				theNotifier.createUpdateChan <- event.service
			} else if event.isDeleted() {
				theNotifier.deleteChan <- event.service
			}
		}
	})
	return theNotifier
}
