package server

import "github.com/glothriel/wormhole/pkg/apps"

// AppListerAdapter implements admin.appLister
type AppListerAdapter struct {
	server *Server
}

// Apps returns a list of apps
func (adapter *AppListerAdapter) Apps() ([]apps.App, error) {
	var allApps []apps.App
	for _, sessionManager := range adapter.server.portExposers {
		allApps = append(allApps, apps.App{
			Port: sessionManager.port,
			Name: sessionManager.appName,
		})
	}
	return allApps, nil
}

// NewServerAppsListAdapter creates ServerAppsListAdapter instances
func NewServerAppsListAdapter(server *Server) *AppListerAdapter {
	return &AppListerAdapter{
		server: server,
	}
}
