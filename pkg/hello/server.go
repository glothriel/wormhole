package hello

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// Server is a separate HTTP server, that allows managing wormhole using API
type Server struct {
	server    *http.Server
	publicKey string
	endpoint  string
	cfg       *wg.Cfg
	cfgWriter *wg.Watcher
	lastIP    net.IP
}

type helloBody struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

func (s *Server) handleHello(w http.ResponseWriter, r *http.Request) {
	ip := nextIP(s.lastIP, 1)
	s.lastIP = ip

	var body helloBody
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		logrus.Errorf("Failed to decode request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.cfg.Peers = append(s.cfg.Peers, wg.Peer{
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

	s.cfgWriter.Update(*s.cfg)
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
	cfg *wg.Cfg,
) *Server {
	mux := mux.NewRouter()
	s := &Server{
		server: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: time.Second * 5,
		},
		publicKey: publicKey,
		endpoint:  endpoint,
		cfg:       cfg,
		lastIP:    nextIP(net.ParseIP(cfg.Address), 1),
		cfgWriter: wg.NewWriter("/storage/wireguard/wg0.conf"),
	}
	mux.HandleFunc("/v1/hello", s.handleHello).Methods(http.MethodPost)
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
