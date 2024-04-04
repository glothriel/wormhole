package hello

import (
	"sync"

	"github.com/glothriel/wormhole/pkg/k8s/svcdetector"
	"github.com/glothriel/wormhole/pkg/peers"
)

type AppStateChangeGenerator struct {
	peerApps map[string][]peers.App

	changes chan svcdetector.AppStateChange
	lock    sync.Mutex
}

func (s *AppStateChangeGenerator) OnSync(peer string, apps []peers.App, syncErr error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	apps = patchPeer(apps, peer)
	oldApps, oldAppsOk := s.peerApps[peer]
	if !oldAppsOk {
		oldApps = make([]peers.App, 0)
	}
	addedApps := make([]peers.App, 0)
	removedApps := make([]peers.App, 0)
	changedApps := make([]peers.App, 0)

	for _, app := range apps {
		if !contains(oldApps, app) {
			addedApps = append(addedApps, app)
		}
	}
	for _, oldApp := range oldApps {
		if !contains(apps, oldApp) {
			removedApps = append(removedApps, oldApp)
		}
	}

	for _, app := range apps {
		for _, oldApp := range oldApps {
			if app.Name == oldApp.Name && app.Address != oldApp.Address {
				changedApps = append(changedApps, app)
			}
		}
	}

	for _, app := range addedApps {
		s.changes <- svcdetector.AppStateChange{
			App:   app,
			State: svcdetector.AppStateChangeAdded,
		}
	}

	for _, app := range removedApps {
		s.changes <- svcdetector.AppStateChange{
			App:   app,
			State: svcdetector.AppStateChangeWithdrawn,
		}
	}

	for _, app := range changedApps {
		s.changes <- svcdetector.AppStateChange{
			App:   app,
			State: svcdetector.AppStateChangeWithdrawn,
		}
		s.changes <- svcdetector.AppStateChange{
			App:   app,
			State: svcdetector.AppStateChangeAdded,
		}
	}

	s.peerApps[peer] = apps

}

func (s *AppStateChangeGenerator) Changes() chan svcdetector.AppStateChange {
	return s.changes
}

func NewAppStateChangeGenerator() *AppStateChangeGenerator {
	return &AppStateChangeGenerator{
		peerApps: make(map[string][]peers.App),
		changes:  make(chan svcdetector.AppStateChange),
	}
}

func contains(apps []peers.App, app peers.App) bool {
	for _, a := range apps {
		if a.Name == app.Name {
			return true
		}
	}
	return false
}

func patchPeer(a []peers.App, peerName string) []peers.App {
	for i := range a {
		a[i] = peers.WithPeer(a[i], peerName)
	}
	return a
}
