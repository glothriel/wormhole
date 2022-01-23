package server

import (
	"github.com/glothriel/wormhole/pkg/admin"
)

// AppListerAdapter implements admin.appLister
type AppListerAdapter struct {
	appExposer AppExposer
}

// Apps returns a list of apps
func (adapter *AppListerAdapter) Apps() ([]admin.AppListEntry, error) {
	allApps := []admin.AppListEntry{}
	for _, theApp := range adapter.appExposer.Apps() {
		allApps = append(allApps, admin.AppListEntry{
			Endpoint: theApp.App.Address,
			App:      theApp.App.Name,
			Peer:     theApp.Peer.Name(),
		})
	}
	return allApps, nil
}

// NewServerAppsListAdapter creates ServerAppsListAdapter instances
func NewServerAppsListAdapter(exposer AppExposer) *AppListerAdapter {
	return &AppListerAdapter{
		appExposer: exposer,
	}
}
