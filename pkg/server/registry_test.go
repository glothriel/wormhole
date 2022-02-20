package server

import (
	"testing"

	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/stretchr/testify/assert"
)

func TestExposedAppRegistryGetReturnsNothingIfRegistryEmpty(t *testing.T) {
	// given
	registry := newExposedAppsRegistry()

	// when
	portOpener, ok := registry.get(peers.NewMockPeer(), peers.App{Name: "foo"})

	// then
	assert.Nil(t, portOpener)
	assert.False(t, ok)
}

func TestExposedAppRegistryGetReturnsItemIfItWasPreviouslyStored(t *testing.T) {
	// given
	registry := newExposedAppsRegistry()
	mockPortOpener := &perAppPortOpener{}
	mockPeer := peers.NewMockPeer()
	mockApp := peers.App{Name: "foo"}

	// when
	registry.store(mockPeer, mockApp, mockPortOpener)
	portOpener, ok := registry.get(mockPeer, mockApp)

	// then
	assert.Equal(t, mockPortOpener, portOpener)
	assert.True(t, ok)
}

func TestExposedAppRegistryItems(t *testing.T) {
	// given
	registry := newExposedAppsRegistry()
	mockPortOpener := &perAppPortOpener{}
	mockPeer := peers.NewMockPeer()
	mockApp := peers.App{Name: "foo"}

	// when
	registry.store(mockPeer, mockApp, mockPortOpener)
	allItems := registry.items()

	// then
	assert.Len(t, allItems, 1)
	assert.Equal(t, mockPeer, allItems[0].peer)
	assert.Equal(t, mockApp, allItems[0].app)
	assert.Equal(t, mockPortOpener, allItems[0].portOpener)
}

func TestExposedAppRegistryDeleteReallyDeletesEntries(t *testing.T) {
	// given
	registry := newExposedAppsRegistry()
	mockPortOpener := &perAppPortOpener{}
	mockPeer := peers.NewMockPeer()
	mockApp := peers.App{Name: "foo"}

	// when
	registry.store(mockPeer, mockApp, mockPortOpener)
	registry.delete(mockPeer, mockApp)

	//then
	portOpener, ok := registry.get(mockPeer, mockApp)
	assert.Nil(t, portOpener)
	assert.False(t, ok)
}
