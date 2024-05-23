package k8s

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/glothriel/wormhole/pkg/listeners"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type k8sServiceExposer struct {
	namespace    string
	child        listeners.Exposer
	ownSelectors map[string]string
}

func (factory *k8sServiceExposer) Add(app peers.App) (peers.App, error) {
	config, inClusterConfigErr := rest.InClusterConfig()
	if inClusterConfigErr != nil {
		return peers.App{}, inClusterConfigErr
	}
	clientset, clientSetErr := kubernetes.NewForConfig(config)
	if clientSetErr != nil {
		return peers.App{}, clientSetErr
	}
	servicesClient := clientset.CoreV1().Services(factory.namespace)
	addedApp, childFactoryErr := factory.child.Add(app)
	if childFactoryErr != nil {
		return peers.App{}, childFactoryErr
	}
	port, portErr := extractPortFromAddr(addedApp.Address)
	if portErr != nil {
		return peers.App{}, multierr.Combine(portErr, factory.Withdraw(addedApp))
	}

	serviceName := fmt.Sprintf("%s-%s", app.Peer, app.Name)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: factory.namespace,
			Labels:    factory.buildLabelsForSvc(),
			Annotations: map[string]string{
				"x-wormhole-app":  app.Name,
				"x-wormhole-peer": app.Peer,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port:       app.OriginalPort,
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
		return peers.App{}, multierr.Combine(fmt.Errorf("Could not get service %s: %v", serviceName, getErr), factory.Withdraw(addedApp))
	} else {
		logrus.Debugf("Updating service %s", serviceName)
		service.SetResourceVersion(previousService.GetResourceVersion())
		_, upsertErr = servicesClient.Update(context.Background(), service, metav1.UpdateOptions{})
	}
	if upsertErr != nil {
		return peers.App{}, multierr.Combine(fmt.Errorf("Unable to upsert the service: %v", upsertErr), factory.Withdraw(addedApp))
	}
	return peers.WithAddress(addedApp, fmt.Sprintf("%s.%s:%d", serviceName, factory.namespace, app.OriginalPort)), nil
}

func (factory *k8sServiceExposer) Withdraw(app peers.App) error {
	serviceName := fmt.Sprintf("%s-%s", app.Peer, app.Name)
	logrus.Debugf("Deleting service %s", serviceName)
	config, inClusterConfigErr := rest.InClusterConfig()
	if inClusterConfigErr != nil {
		return inClusterConfigErr
	}
	clientset, clientSetErr := kubernetes.NewForConfig(config)
	if clientSetErr != nil {
		return clientSetErr
	}
	servicesClient := clientset.CoreV1().Services(factory.namespace)
	deleteErr := servicesClient.Delete(context.Background(), serviceName, metav1.DeleteOptions{})
	if deleteErr != nil {
		return fmt.Errorf("Could not delete service %s: %v", serviceName, deleteErr)
	}
	return factory.child.Withdraw(app)
}

func (factory *k8sServiceExposer) WithdrawAll() error {
	return nil
}

func (factory *k8sServiceExposer) buildLabelsForSvc() map[string]string {
	labelsMap := map[string]string{}
	for sKey, sVal := range factory.ownSelectors {
		labelsMap[sKey] = sVal
	}
	return labelsMap
}

// NewK8sExposer implements PortOpenerFactory as a decorator over existing PortOpenerFactory, that
// also creates kubernetes service for given opened port
func NewK8sExposer(
	namespace string,
	selectors map[string]string,
	childFactory listeners.Exposer,
) listeners.Exposer {
	return &k8sServiceExposer{
		namespace:    namespace,
		ownSelectors: selectors,
		child:        childFactory,
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
