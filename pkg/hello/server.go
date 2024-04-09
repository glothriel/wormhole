package hello

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type helloResult struct {
}

type appRegistry interface {
	Apps() []peers.App
}

// Server is a separate HTTP server, that allows managing wormhole using API
type Server struct {
	extServer *http.Server
	intServer *http.Server
	publicKey string
	endpoint  string
	cfg       *wg.Config
	cfgWriter *wg.Watcher
	lastIP    net.IP
	m         sync.Mutex

	apps             appRegistry
	hostnamesToNames map[string]string

	remoteNginxAdapter *AppStateChangeGenerator
}

func (s *Server) handleHello(w http.ResponseWriter, r *http.Request) {
	s.m.Lock()
	ip := nextIP(s.lastIP, 1)
	s.lastIP = ip
	s.m.Unlock()

	var body helloRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		logrus.Errorf("Failed to decode request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s.hostnamesToNames[ip.String()] = body.Name

	s.cfg.Upsert(wg.Peer{
		PublicKey:  body.PublicKey,
		AllowedIPs: fmt.Sprintf("%s/32,%s/32", ip.String(), s.cfg.Address),
	})
	logrus.Infof("Registered new peer: %s, %s", body.Name, ip.String())

	theResponse := map[string]any{
		"peer": map[string]any{
			"public_key": s.publicKey,
			"endpoint":   s.endpoint,
		},
		"peer_ip":    ip.String(),
		"gateway_ip": s.cfg.Address,
	}

	responseBody, marshalErr := json.Marshal(theResponse)
	if marshalErr != nil {
		logrus.Errorf("Failed to marshal response: %v", marshalErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseBody)

	updateErr := s.cfgWriter.Update(*s.cfg)
	if updateErr != nil {
		logrus.Errorf("Failed to update config: %v", updateErr)
	}

}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {

	var body syncRequestAndResponse
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		logrus.Errorf("Failed to decode request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	segments := strings.Split(r.RemoteAddr, ":")
	mappedName, ok := s.hostnamesToNames[segments[0]]
	if !ok {
		logrus.Errorf("No hostname found for %s", segments[0])
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s.remoteNginxAdapter.OnSync(
		mappedName,
		toPeerApps(mappedName, segments[0], body.Apps),
		nil,
	)

	apps := []syncRequestApp{}
	for _, app := range s.apps.Apps() {
		port, parseErr := strconv.Atoi(strings.Split(app.Address, ":")[1])
		if parseErr != nil {
			logrus.Errorf("Failed to parse port: %v", parseErr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		apps = append(apps, syncRequestApp{
			Name:         app.Name,
			Peer:         app.Peer,
			Port:         port,
			OriginalPort: app.OriginalPort,
			TargetLabels: app.TargetLabels,
		})
	}
	reqBodyJSON := syncRequestAndResponse{
		Apps: apps,
	}
	respBody, marshalErr := json.Marshal(reqBodyJSON)
	if marshalErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(respBody)

}

// Listen starts the server
func (apiServer *Server) Listen() error {
	apiServer.cfgWriter.Update(*apiServer.cfg)
	go func() {
		logrus.Infof("Starting internal server on %s", apiServer.intServer.Addr)
		retry.Do(func() error {
			// The address will bind only after wireguard is up, hence the retry
			listenErr := apiServer.intServer.ListenAndServe()
			if listenErr != nil {
				logrus.Errorf("Failed to start internal server: %v, will retry", listenErr)
			}
			return listenErr
		},
			retry.Attempts(0), // infinite retries
			retry.DelayType(retry.BackOffDelay),
			retry.MaxDelay(time.Second*10),
			retry.Delay(time.Millisecond*100),
		)
	}()
	logrus.Infof("Starting external server on %s", apiServer.extServer.Addr)
	return apiServer.extServer.ListenAndServe()
}

// NewServer creates WormholeAdminServer instances
func NewServer(
	intAddr string,
	extAddr string,
	publicKey string,
	endpoint string,
	cfg *wg.Config,
	apps appRegistry,
	remoteNginxAdapter *AppStateChangeGenerator,
	wgWatcher *wg.Watcher,
) *Server {
	extMux := mux.NewRouter()
	intMux := mux.NewRouter()
	s := &Server{
		extServer: &http.Server{
			Addr:              extAddr,
			Handler:           extMux,
			ReadHeaderTimeout: time.Second * 5,
		},
		intServer: &http.Server{
			Addr:              intAddr,
			Handler:           intMux,
			ReadHeaderTimeout: time.Second * 5,
		},
		publicKey:          publicKey,
		endpoint:           endpoint,
		cfg:                cfg,
		lastIP:             nextIP(net.ParseIP(cfg.Address), 1),
		cfgWriter:          wgWatcher,
		m:                  sync.Mutex{},
		apps:               apps,
		remoteNginxAdapter: remoteNginxAdapter,
		hostnamesToNames:   map[string]string{},
	}
	extMux.HandleFunc("/v1/hello", s.handleHello).Methods(http.MethodPost)
	intMux.HandleFunc("/v1/sync", s.handleSync).Methods(http.MethodPost)
	return s
}

func nextIP(ip net.IP, inc uint) net.IP {
	i := ip.To4()
	v := uint(i[0])<<24 + uint(i[1])<<16 + uint(i[2])<<8 + uint(i[3])
	v += inc
	v3 := byte(v & 0xFF)
	v2 := byte((v >> 8) & 0xFF)
	v1 := byte((v >> 16) & 0xFF)
	v0 := byte((v >> 24) & 0xFF)
	return net.IPv4(v0, v1, v2, v3)
}

func toPeerApps(peerName, hostname string, s []syncRequestApp) []peers.App {
	apps := make([]peers.App, 0, len(s))
	for _, app := range s {
		apps = append(apps, peers.App{
			Name:         app.Name,
			Peer:         peerName,
			Address:      fmt.Sprintf("%s:%d", hostname, app.Port),
			OriginalPort: app.OriginalPort,
			TargetLabels: app.TargetLabels,
		})
	}
	return apps
}
