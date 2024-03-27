package hello

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/glothriel/wormhole/pkg/nginx"
	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// Server is a separate HTTP server, that allows managing wormhole using API
type Server struct {
	server    *http.Server
	publicKey string
	endpoint  string
	cfg       *wg.Config
	cfgWriter *wg.Watcher
	lastIP    net.IP
	m         sync.Mutex

	nginxConfig *nginx.ConfigGuard
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

	s.cfg.Upsert(wg.Peer{
		PublicKey:  body.PublicKey,
		AllowedIPs: ip.String() + "/32," + s.cfg.Address + "/32",
	})

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
	s.m.Lock()
	ip := nextIP(s.lastIP, 1)
	s.lastIP = ip
	s.m.Unlock()

	var body syncRequestAndResponse
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		logrus.Errorf("Failed to decode request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logrus.Infof("Received sync request: %v, %s", body, r.RemoteAddr)

	apps := []syncRequestApp{}
	for _, server := range s.nginxConfig.Servers {
		apps = append(apps, syncRequestApp{
			Name: server.App.Name,
			Peer: server.App.Peer,
			Port: server.ListenPort,
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
	return apiServer.server.ListenAndServe()
}

// NewServer creates WormholeAdminServer instances
func NewServer(
	addr string,
	publicKey string,
	endpoint string,
	cfg *wg.Config,
	nginxConfig *nginx.ConfigGuard,
) *Server {
	mux := mux.NewRouter()
	s := &Server{
		server: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: time.Second * 5,
		},
		publicKey:   publicKey,
		endpoint:    endpoint,
		cfg:         cfg,
		lastIP:      nextIP(net.ParseIP(cfg.Address), 1),
		cfgWriter:   wg.NewWriter("/storage/wireguard/wg0.conf"),
		m:           sync.Mutex{},
		nginxConfig: nginxConfig,
	}
	mux.HandleFunc("/v1/hello", s.handleHello).Methods(http.MethodPost)
	mux.HandleFunc("/v1/sync", s.handleSync).Methods(http.MethodPost)
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
