package svcdetector

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
	go func() {
		for event := range repository.watch() {
			if event.isAddedOrModified() {
				theNotifier.createUpdateChan <- event.service
			} else if event.isDeleted() {
				theNotifier.deleteChan <- event.service
			}
		}
	}()
	return theNotifier
}
