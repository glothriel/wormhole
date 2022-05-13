package svcdetector

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	eventTypeAddedOrModified = iota
	eventTypeDeleted
)

// ServiceRepository allows quering k8s server for services
type ServiceRepository interface {
	list() ([]serviceWrapper, error)
	watch() chan watchEvent
}

type watchEvent struct {
	evtType int
	service serviceWrapper
}

func (event watchEvent) isAddedOrModified() bool {
	return event.evtType == eventTypeAddedOrModified
}

func (event watchEvent) isDeleted() bool {
	return event.evtType == eventTypeDeleted
}

type defaultServiceRepository struct {
	client clientcorev1.ServiceInterface
}

func (repository defaultServiceRepository) list() ([]serviceWrapper, error) {
	services := []serviceWrapper{}
	k8sServices, listErr := repository.client.List(context.Background(), v1.ListOptions{})
	if listErr != nil {
		return []serviceWrapper{}, listErr
	}
	for i := range k8sServices.Items {
		services = append(services, newDefaultServiceWrapper(&k8sServices.Items[i]))
	}
	return services, nil
}

func (repository defaultServiceRepository) watch() chan watchEvent {
	theChannel := make(chan watchEvent)
	go func() {
		for {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				time.Second*30,
			)
			watcher, watchErr := repository.client.Watch(ctx, v1.ListOptions{})
			if watchErr != nil {
				logrus.Errorf("Failed to watch for kubernetes services, none will be exposed: %v", watchErr)
				time.Sleep(time.Second * 5)
				cancel()
				continue
			}
			for event := range watcher.ResultChan() {
				svc, castingOK := event.Object.(*corev1.Service)
				if !castingOK {
					cancel()
					continue
				}
				if event.Type == watch.Added || event.Type == watch.Modified {
					theChannel <- watchEvent{
						evtType: eventTypeAddedOrModified,
						service: newDefaultServiceWrapper(svc),
					}
				} else if event.Type == watch.Deleted {
					theChannel <- watchEvent{
						evtType: eventTypeDeleted,
						service: newDefaultServiceWrapper(svc),
					}
				}
			}
			cancel()
		}
	}()
	return theChannel
}

// NewDefaultServiceRepository creates ServiceRepository instances
func NewDefaultServiceRepository(client clientcorev1.ServiceInterface) ServiceRepository {
	return &defaultServiceRepository{
		client: client,
	}
}
