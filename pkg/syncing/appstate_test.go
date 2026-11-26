package syncing

import (
	"testing"

	"github.com/glothriel/wormhole/pkg/apps"
	"github.com/glothriel/wormhole/pkg/k8s/svcdetector"
	"github.com/stretchr/testify/assert"
)

// collectChanges runs f and returns every change the generator emitted while f ran
func collectChanges(g *AppStateChangeGenerator, f func()) []svcdetector.AppStateChange {
	done := make(chan struct{})
	go func() {
		f()
		close(done)
	}()
	changes := make([]svcdetector.AppStateChange, 0)
	for {
		select {
		case change := <-g.Changes():
			changes = append(changes, change)
		case <-done:
			return changes
		}
	}
}

func TestAppStateChangeGeneratorRemovePeer(t *testing.T) {
	g := NewAppStateChangeGenerator()
	collectChanges(g, func() {
		g.UpdateForPeer("peer1", []apps.App{{Name: "app", Address: "a:1"}, {Name: "other", Address: "a:2"}})
		g.UpdateForPeer("peer2", []apps.App{{Name: "app", Address: "b:1"}})
	})

	changes := collectChanges(g, func() { g.RemovePeer("peer1") })

	assert.ElementsMatch(t, []svcdetector.AppStateChange{
		{App: apps.App{Name: "app", Address: "a:1", Peer: "peer1"}, State: svcdetector.AppStateChangeWithdrawn},
		{App: apps.App{Name: "other", Address: "a:2", Peer: "peer1"}, State: svcdetector.AppStateChangeWithdrawn},
	}, changes)

	changes = collectChanges(g, func() {
		g.UpdateForPeer("peer1", []apps.App{{Name: "app", Address: "a:1"}})
	})

	assert.Equal(t, []svcdetector.AppStateChange{
		{App: apps.App{Name: "app", Address: "a:1", Peer: "peer1"}, State: svcdetector.AppStateChangeAdded},
	}, changes)

	changes = collectChanges(g, func() {
		g.UpdateForPeer("peer2", []apps.App{{Name: "app", Address: "b:1"}})
	})

	assert.Empty(t, changes)
}

func TestAppStateChangeGeneratorRemoveUnknownPeer(t *testing.T) {
	g := NewAppStateChangeGenerator()

	changes := collectChanges(g, func() { g.RemovePeer("unknown") })

	assert.Empty(t, changes)
}
