package svcdetector

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
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
	client dynamic.Interface
}

func (repository defaultServiceRepository) list() ([]serviceWrapper, error) {
	services := []serviceWrapper{}
	k8sServices, listErr := repository.client.Resource(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}).List(context.Background(), v1.ListOptions{})
	if listErr != nil {
		return []serviceWrapper{}, listErr
	}
	for i := range k8sServices.Items {
		svc := &corev1.Service{}
		if convertError := runtime.DefaultUnstructuredConverter.FromUnstructured(
			k8sServices.Items[i].Object, svc,
		); convertError != nil {
			return services, fmt.Errorf(
				"Received invalid type when trying to dispatch informer events: %v",
				convertError,
			)
		}
		services = append(services, newDefaultServiceWrapper(svc))
	}
	return services, nil
}

func (repository defaultServiceRepository) watch() chan watchEvent {
	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		repository.client,
		time.Second*10,
		metav1.NamespaceAll,
		nil,
	)
	informer := informerFactory.ForResource(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	})
	theChannel := make(chan watchEvent)
	go func() {
		stopCh := make(chan struct{})
		go func(stopCh <-chan struct{}, s cache.SharedIndexInformer) {
			handlers := cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj any) {
					for _, event := range repository.onAddedOrModified(obj) {
						theChannel <- event
					}
				},
				UpdateFunc: func(_, obj any) {
					for _, event := range repository.onAddedOrModified(obj) {
						theChannel <- event
					}
				},
				DeleteFunc: func(obj any) {
					for _, event := range repository.onDeleted(obj) {
						theChannel <- event
					}
				},
			}
			_, addEventHandlerErr := s.AddEventHandler(handlers)
			if addEventHandlerErr != nil {
				return
			}
			s.Run(stopCh)
		}(stopCh, informer.Informer())
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh
		close(stopCh)
	}()
	return theChannel
}

func (repository defaultServiceRepository) onAddedOrModified(informerObject any) []watchEvent {
	return repository.dispatchEvents(eventTypeAddedOrModified, informerObject)
}

func (repository defaultServiceRepository) onDeleted(informerObject any) []watchEvent {
	return repository.dispatchEvents(eventTypeDeleted, informerObject)
}

func (repository defaultServiceRepository) dispatchEvents(eventType int, informerObject any) []watchEvent {
	u := informerObject.(*unstructured.Unstructured)
	svc := corev1.Service{}
	if convertError := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &svc); convertError != nil {
		logrus.Errorf("Received invalid type when trying to dispatch informer events: %v", convertError)
		return []watchEvent{}
	}
	return []watchEvent{
		{
			evtType: eventType,
			service: newDefaultServiceWrapper(&svc),
		},
	}
}

// NewDefaultServiceRepository creates ServiceRepository instances
func NewDefaultServiceRepository(client dynamic.Interface) ServiceRepository {
	return &defaultServiceRepository{
		client: client,
	}
}
