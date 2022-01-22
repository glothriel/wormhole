package admin

import (
	"net/http"
)

// WormholeAdminServer is a separate HTTP server, that allows managing wormhole using API
type WormholeAdminServer struct {
	server *http.Server
}

// Listen starts the server
func (apiServer *WormholeAdminServer) Listen() error {
	return apiServer.server.ListenAndServe()
}

// NewWormholeAdminServer creates WormholeAdminServer instances
func NewWormholeAdminServer(addr string, appList appLister) *WormholeAdminServer {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/apps", listAppsHander(appList))
	return &WormholeAdminServer{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
}
