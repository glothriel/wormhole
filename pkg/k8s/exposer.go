// Package k8s implements exposer for kubernetes services
package k8s

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glothriel/wormhole/pkg/apps"
	"github.com/glothriel/wormhole/pkg/listeners"
	"go.uber.org/multierr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type clientProvider interface {
	New() (*kubernetes.Clientset, error)
}

type fromInClusterConfigClientProvider struct{}

func (fromInClusterConfigClientProvider) New() (*kubernetes.Clientset, error) {
	config, inClusterConfigErr := rest.InClusterConfig()
	if inClusterConfigErr != nil {
		return nil, inClusterConfigErr
	}
	return kubernetes.NewForConfig(config)
}

type k8sResourceExposer struct {
	namespace string
	child     listeners.Exposer
	selectors map[string]string

	managedResources []managedK8sResource

	clientProvider clientProvider
}

func (exp *k8sResourceExposer) Add(app apps.App) (apps.App, error) {
	clientset, clientSetErr := exp.clientProvider.New()
	if clientSetErr != nil {
		return apps.App{}, clientSetErr
	}
	addedApp, childFactoryErr := exp.child.Add(app)
	if childFactoryErr != nil {
		return apps.App{}, childFactoryErr
	}
	entityName := fmt.Sprintf("%s-%s", app.Peer, app.Name)
	for _, managedResource := range exp.managedResources {
		addErr := managedResource.Add(k8sResourceMetadata{
			entityName:       entityName,
			initialApp:       app,
			childReturnedApp: addedApp,
		}, clientset)
		if addErr != nil {
			return apps.App{}, multierr.Combine(addErr, exp.child.Withdraw(app))
		}
	}
	return apps.WithAddress(addedApp, fmt.Sprintf("%s.%s:%d", entityName, exp.namespace, app.OriginalPort)), nil
}

func (exp *k8sResourceExposer) Withdraw(app apps.App) error {
	clientset, clientSetErr := exp.clientProvider.New()
	if clientSetErr != nil {
		return clientSetErr
	}
	entityName := fmt.Sprintf("%s-%s", app.Peer, app.Name)
	for i := range exp.managedResources {
		managedResource := exp.managedResources[len(exp.managedResources)-1-i]
		removeErr := managedResource.Remove(entityName, clientset)
		if removeErr != nil {
			return removeErr
		}
	}
	return exp.child.Withdraw(app)
}

func (exp *k8sResourceExposer) WithdrawAll() error {
	clientset, clientSetErr := exp.clientProvider.New()
	if clientSetErr != nil {
		return clientSetErr
	}
	for i := range exp.managedResources {
		managedResource := exp.managedResources[len(exp.managedResources)-1-i]
		removeAllErr := managedResource.RemoveAll(clientset)
		if removeAllErr != nil {
			return removeAllErr
		}
	}
	return nil
}

// NewK8sExposer implements PortOpenerFactory as a decorator over existing PortOpenerFactory, that
// also creates kubernetes service for given opened port
func NewK8sExposer(
	namespace string,
	selectors map[string]string,
	enableNetworkPolicies bool,
	childExposer listeners.Exposer,
) listeners.Exposer {
	resources := []managedK8sResource{}
	if enableNetworkPolicies {
		resources = append(resources, newManagedK8sNetworkPolicy(namespace, selectors))
	}
	return &k8sResourceExposer{
		namespace:      namespace,
		selectors:      selectors,
		child:          childExposer,
		clientProvider: fromInClusterConfigClientProvider{},

		managedResources: append(resources, newManagedK8sService(namespace, selectors)),
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

const exposedByLabel = "wormhole.glothriel.github.com/exposed-by"
const exposedAppLabel = "wormhole.glothriel.github.com/exposed-app"
const exposedPeerLabel = "wormhole.glothriel.github.com/exposed-peer"

func resourceLabels(app apps.App) map[string]string {
	return map[string]string{
		exposedByLabel:   "wormhole",
		exposedAppLabel:  app.Name,
		exposedPeerLabel: app.Peer,
	}
}

type k8sResourceMetadata struct {
	entityName       string
	initialApp       apps.App
	childReturnedApp apps.App
}

type managedK8sResource interface {
	Add(k8sResourceMetadata, *kubernetes.Clientset) error
	Remove(name string, clientset *kubernetes.Clientset) error
	RemoveAll(*kubernetes.Clientset) error
}

func extractPortFromAddr(address string) (int, error) {
	parts := strings.Split(address, ":")
	return strconv.Atoi(parts[1])
}
