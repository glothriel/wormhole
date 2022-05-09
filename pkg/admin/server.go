package admin

import (
	"net/http"

	"github.com/gorilla/mux"
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
func NewWormholeAdminServer(
	addr string,
	appList appLister,
	gatherer *ConsentGatherer,
) *WormholeAdminServer {
	mux := mux.NewRouter()
	mux.HandleFunc("/v1/apps", listAppsHander(appList))
	mux.HandleFunc("/v1/requests", listAcceptRequests(gatherer))
	mux.HandleFunc("/v1/requests/{fingerprint}", updateAcceptRequest(gatherer))
	return &WormholeAdminServer{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
}
