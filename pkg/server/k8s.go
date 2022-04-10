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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type k8sServicePortOpener struct {
	client      clientcorev1.ServiceInterface
	childOpener portOpener
	serviceName string
}

func (sm *k8sServicePortOpener) connections() chan appConnection {
	return sm.childOpener.connections()
}

func (sm *k8sServicePortOpener) listenAddr() string {
	return fmt.Sprintf("%s:%d", sm.serviceName, 8080)
}

func (sm *k8sServicePortOpener) close() error {
	logrus.Debugf("Deleting service %s", sm.serviceName)
	return multierr.Combine(
		sm.childOpener.close(),
		sm.client.Delete(context.Background(), sm.serviceName, metav1.DeleteOptions{}),
	)
}

type k8sServicePortOpenerFactory struct {
	namespace    string
	childFactory PortOpenerFactory
	ownSelectors map[string]string
}

func (factory *k8sServicePortOpenerFactory) Create(app peers.App, peer peers.Peer) (portOpener, error) {
	config, inClusterConfigErr := rest.InClusterConfig()
	if inClusterConfigErr != nil {
		return nil, inClusterConfigErr
	}
	// creates the clientset
	clientset, clientSetErr := kubernetes.NewForConfig(config)
	if clientSetErr != nil {
		return nil, clientSetErr
	}
	servicesClient := clientset.CoreV1().Services(factory.namespace)
	childOpener, childFactoryErr := factory.childFactory.Create(app, peer)
	if childFactoryErr != nil {
		return nil, childFactoryErr
	}
	port, portErr := strconv.Atoi(strings.Split(childOpener.listenAddr(), ":")[1])
	if portErr != nil {
		return nil, multierr.Combine(portErr, childOpener.close())
	}
	labelsMap := map[string]string{}
	for sKey, sVal := range factory.ownSelectors {
		labelsMap[sKey] = sVal
	}
	originalPort, originalPortErr := extractPortFromAddr(app.Address)
	if portErr != nil {
		return nil, multierr.Combine(originalPortErr, childOpener.close())
	}
	serviceName := fmt.Sprintf("%s-%s", peer.Name(), app.Name)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: factory.namespace,
			Labels:    labelsMap,
			Annotations: map[string]string{
				"x-wormhole-app":  app.Name,
				"x-wormhole-peer": peer.Name(),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port:       int32(originalPort),
				TargetPort: intstr.FromInt(port),
			}},
			Selector: factory.ownSelectors,
		},
	}
	var upsertErr error
	previousService, getErr := servicesClient.Get(context.Background(), serviceName, metav1.GetOptions{})
	if errors.IsNotFound(getErr) {
		logrus.Debugf("Creating service %s", serviceName)
		_, upsertErr = servicesClient.Create(context.Background(), service, metav1.CreateOptions{})
	} else if getErr != nil {
		return nil, multierr.Combine(fmt.Errorf("Could not get service %s: %v", serviceName, getErr), childOpener.close())
	} else {
		logrus.Debugf("Updating service %s", serviceName)
		service.SetResourceVersion(previousService.GetResourceVersion())
		_, upsertErr = servicesClient.Update(context.Background(), service, metav1.UpdateOptions{})
	}
	if upsertErr != nil {
		return nil, multierr.Combine(fmt.Errorf("Unable to upert the service: %v", upsertErr), childOpener.close())
	}
	return &k8sServicePortOpener{
		serviceName: serviceName,
		client:      servicesClient,
		childOpener: childOpener,
	}, nil
}

// NewK8sServicePortOpenerFactory implements PortOpenerFactory as a decorator over existing PortOpenerFactory, that
// also creates kubernetes service for given opened port
func NewK8sServicePortOpenerFactory(
	namespace string,
	selectors map[string]string,
	childFactory PortOpenerFactory,
) PortOpenerFactory {
	return &k8sServicePortOpenerFactory{
		namespace:    namespace,
		ownSelectors: selectors,
		childFactory: childFactory,
	}
}

// CSVToMap converts key1=v1,key2=v2 entries into flat string map
func CSVToMap(csv string) map[string]string {
	theMap := map[string]string{}
	for _, kvPair := range strings.Split(csv, ",") {
		parsedKVPair := strings.Split(kvPair, "=")
		theMap[parsedKVPair[0]] = strings.Join(parsedKVPair[1:], "=")
	}
	return theMap
}

func extractPortFromAddr(address string) (int, error) {
	parts := strings.Split(address, ":")
	return strconv.Atoi(parts[1])
}
