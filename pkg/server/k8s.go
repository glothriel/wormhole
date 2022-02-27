package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type k8sServicePortOpener struct {
	client      clientcorev1.ServiceInterface
	childOpener portOpener
	service     *corev1.Service
}

func (sm *k8sServicePortOpener) connections() chan appConnection {
	return sm.childOpener.connections()
}

func (sm *k8sServicePortOpener) listenAddr() string {
	return fmt.Sprintf("%s:%d", sm.service.ObjectMeta.Name, 8080)
}

func (sm *k8sServicePortOpener) close() error {
	return multierr.Combine(
		sm.childOpener.close(),
		sm.client.Delete(context.Background(), sm.service.ObjectMeta.Name, metav1.DeleteOptions{}),
	)
}

type k8sServicePortOpenerFactory struct {
	namespace    string
	childFactory PortOpenerFactory
	ownSelectors map[string]string
}

func (factory *k8sServicePortOpenerFactory) Create(app peers.App, peer peers.Peer) (portOpener, error) {
	childOpener, childFactoryErr := factory.childFactory.Create(app, peer)
	if childFactoryErr != nil {
		return nil, childFactoryErr
	}

	config, inClusterConfigErr := rest.InClusterConfig()
	if inClusterConfigErr != nil {
		if closeErr := childOpener.close(); closeErr != nil {
			logrus.Warningf("Failed to close port opener: %v", closeErr)
		}
		return nil, inClusterConfigErr
	}
	// creates the clientset
	clientset, clientSetErr := kubernetes.NewForConfig(config)
	if clientSetErr != nil {
		if closeErr := childOpener.close(); closeErr != nil {
			logrus.Warningf("Failed to close port opener: %v", closeErr)
		}
		return nil, clientSetErr
	}
	servicesClient := clientset.CoreV1().Services(factory.namespace)
	port, portErr := strconv.Atoi(strings.Split(childOpener.listenAddr(), ":")[1])
	if portErr != nil {
		if closeErr := childOpener.close(); closeErr != nil {
			logrus.Warningf("Failed to close port opener: %v", closeErr)
		}
		return nil, portErr
	}
	theMap := map[string]string{
		"app":  app.Name,
		"peer": peer.Name(),
	}
	for sKey, sVal := range factory.ownSelectors {
		theMap[sKey] = sVal
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", peer.Name(), app.Name),
			Namespace: factory.namespace,
			Labels:    theMap,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       8080,
					TargetPort: intstr.FromInt(port),
				},
			},
			Selector: factory.ownSelectors,
		},
	}

	_, createErr := servicesClient.Create(context.Background(), service, metav1.CreateOptions{})
	if createErr != nil {
		if closeErr := childOpener.close(); closeErr != nil {
			logrus.Warningf("Failed to close port opener: %v", closeErr)
		}
		return nil, createErr
	}

	return &k8sServicePortOpener{
		service:     service,
		client:      servicesClient,
		childOpener: childOpener,
	}, nil
}

func NewK8sServicePortOpenerFactory(
	namespace string,
	selectors map[string]string,
	childFactory PortOpenerFactory,
) PortOpenerFactory {
	return &k8sServicePortOpenerFactory{
		namespace:    "default",
		ownSelectors: selectors,
		childFactory: childFactory,
	}
}

func CSVToMap(csv string) map[string]string {
	theMap := map[string]string{}
	for _, kvPair := range strings.Split(csv, ",") {
		parsedKVPair := strings.Split(kvPair, "=")
		theMap[parsedKVPair[0]] = strings.Join(parsedKVPair[1:], "=")
	}
	return theMap
}
