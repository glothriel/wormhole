// Package k8s implements exposer for kubernetes services
package k8s

import (
	"sync"
	"testing"

	"github.com/glothriel/wormhole/pkg/apps"
	"github.com/glothriel/wormhole/pkg/listeners"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
)

type counter struct {
	l    sync.Mutex
	last int
}

func (c *counter) next() int {
	c.l.Lock()
	defer c.l.Unlock()
	r := c.last
	c.last++
	return r
}

type mockClientProvider struct {
}

func (mockClientProvider) New() (*kubernetes.Clientset, error) {
	return &kubernetes.Clientset{}, nil
}

type managedMockResource struct {
	addCalled            int
	addLastCalledWith    k8sResourceMetadata
	addErr               error
	removeCalled         int
	removeLastCalledWith string
	removeErr            error
	removeAllCalled      int
	removeAllErr         error

	// tracks the order of calls, possibly for different resources
	counter *counter
}

func (m *managedMockResource) Add(metadata k8sResourceMetadata, _ *kubernetes.Clientset) error {
	m.addCalled = m.counter.next()
	m.addLastCalledWith = metadata
	return m.addErr
}

func (m *managedMockResource) Remove(entityName string, _ *kubernetes.Clientset) error {
	m.removeCalled = m.counter.next()
	m.removeLastCalledWith = entityName
	return m.removeErr
}

func (m *managedMockResource) RemoveAll(_ *kubernetes.Clientset) error {
	m.removeAllCalled = m.counter.next()
	return m.removeAllErr
}

func TestExposerAdd(t *testing.T) {
	// given
	exposer := NewK8sExposer(
		"namespace",
		map[string]string{},
		false,
		listeners.NewNoOpExposer(),
	).(*k8sResourceExposer)
	exposer.clientProvider = mockClientProvider{}
	counter := &counter{}
	rsc1 := &managedMockResource{counter: counter}
	rsc2 := &managedMockResource{counter: counter}
	exposer.managedResources = []managedK8sResource{rsc1, rsc2}

	// when
	newApp, err := exposer.Add(apps.App{
		Name:         "nginxname",
		Peer:         "nginxpeer",
		OriginalPort: 80,
	})

	// then
	assert.NoError(t, err)
	assert.Equal(t, 0, rsc1.addCalled)
	assert.Equal(t, 1, rsc2.addCalled)
	assert.Equal(t, k8sResourceMetadata{
		entityName:       "nginxpeer-nginxname",
		initialApp:       apps.App{Name: "nginxname", Peer: "nginxpeer", OriginalPort: 80},
		childReturnedApp: apps.App{Name: "nginxname", Peer: "nginxpeer", OriginalPort: 80},
	}, rsc2.addLastCalledWith)
	assert.Equal(t, apps.App{
		Name:         "nginxname",
		Peer:         "nginxpeer",
		OriginalPort: 80,
		Address:      "nginxpeer-nginxname.namespace:80",
	}, newApp)
}

func TestExposerWithdraw(t *testing.T) {
	// given
	exposer := NewK8sExposer(
		"namespace",
		map[string]string{},
		false,
		listeners.NewNoOpExposer(),
	).(*k8sResourceExposer)
	exposer.clientProvider = mockClientProvider{}
	counter := &counter{}
	rsc1 := &managedMockResource{counter: counter}
	rsc2 := &managedMockResource{counter: counter}
	exposer.managedResources = []managedK8sResource{rsc1, rsc2}

	// when
	err := exposer.Withdraw(apps.App{})

	// then
	assert.NoError(t, err)

	// order should be reversed
	assert.Equal(t, 0, rsc2.removeCalled)
	assert.Equal(t, 1, rsc1.removeCalled)
}

func TestExposerWithdrawAll(t *testing.T) {
	// given
	exposer := NewK8sExposer(
		"namespace",
		map[string]string{},
		false,
		listeners.NewNoOpExposer(),
	).(*k8sResourceExposer)
	exposer.clientProvider = mockClientProvider{}
	counter := &counter{}
	rsc1 := &managedMockResource{counter: counter}
	rsc2 := &managedMockResource{counter: counter}
	exposer.managedResources = []managedK8sResource{rsc1, rsc2}

	// when
	err := exposer.WithdrawAll()

	// then
	assert.NoError(t, err)

	// order should be reversed
	assert.Equal(t, 0, rsc2.removeAllCalled)
	assert.Equal(t, 1, rsc1.removeAllCalled)
}
