// Package k8s implements exposer for kubernetes services
package k8s

import (
	"crypto/sha256"
	"encoding/hex"
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
	entityName := capName(fmt.Sprintf("%s-%s", app.Peer, app.Name))
	for _, managedResource := range exp.managedResources {
		addErr := managedResource.Add(k8sResourceMetadata{
			entityName:      entityName,
			originalApp:     app,
			afterExposedApp: addedApp,
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
	entityName := capName(fmt.Sprintf("%s-%s", app.Peer, app.Name))
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

type k8sResourceMetadata struct {
	entityName      string
	originalApp     apps.App
	afterExposedApp apps.App
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

func capName(name string) string {
	if len(name) <= 63 {
		return name
	}

	// Calculate hash of the full name
	hasher := sha256.New()
	hasher.Write([]byte(name))
	hash := hex.EncodeToString(hasher.Sum(nil))[:8] // Take first 8 chars of hash

	// We need 9 chars for "-" + hash
	// So we can search up to position 54 (63 - 9 = 54)
	searchStart := 32
	searchEnd := 54

	substring := name[searchStart:searchEnd]
	hyphenIndex := strings.LastIndex(substring, "-")

	if hyphenIndex != -1 {
		// Found hyphen in the search range
		actualIndex := searchStart + hyphenIndex
		return name[:actualIndex] + "-" + hash
	}

	// No suitable hyphen found, cut at position 54
	return name[:54] + "-" + hash
}
