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
	theSpec := corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Port:       8080,
				TargetPort: intstr.FromInt(port),
			},
		},
		Selector: factory.ownSelectors,
	}

	serviceName := fmt.Sprintf("%s-%s", peer.Name(), app.Name)

	var upsertErr error

	service, getErr := servicesClient.Get(context.Background(), serviceName, metav1.GetOptions{})

	if errors.IsNotFound(getErr) {
		_, upsertErr = servicesClient.Create(context.Background(), &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:            serviceName,
				Namespace:       factory.namespace,
				ResourceVersion: "1000",
				Labels:          theMap,
			},
			Spec: theSpec,
		}, metav1.CreateOptions{})
	} else {
		service.Spec = theSpec
		resourceAtNum, atoiErr := strconv.Atoi(service.GetResourceVersion())
		if atoiErr != nil {
			resourceAtNum = 1000
		}
		service.SetResourceVersion(strconv.Itoa(resourceAtNum + 1))
		_, upsertErr = servicesClient.Update(context.Background(), service, metav1.UpdateOptions{})
	}

	if upsertErr != nil {
		if closeErr := childOpener.close(); closeErr != nil {
			logrus.Warningf("Failed to close port opener: %v", closeErr)
		}
		return nil, fmt.Errorf("Unable to upert the service: %v", upsertErr)
	}

	return &k8sServicePortOpener{
		serviceName: serviceName,
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
