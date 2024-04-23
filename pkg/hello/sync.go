package hello

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
)

type SyncEncoder interface {
	Encode([]peers.App) ([]byte, error)
	Decode([]byte) ([]peers.App, error)
}

type jsonSyncEncoder struct{}

func (e *jsonSyncEncoder) Encode(apps []peers.App) ([]byte, error) {
	return json.Marshal(apps)
}

func (e *jsonSyncEncoder) Decode(data []byte) ([]peers.App, error) {
	var apps []peers.App
	err := json.Unmarshal(data, &apps)
	return apps, err
}

func NewJSONSyncEncoder() SyncEncoder {
	return &jsonSyncEncoder{}
}

type IncomingSyncRequest struct {
	Request  []byte
	Response chan []byte
	Err      chan error
}

type SyncClientTransport interface {
	Sync([]byte) ([]byte, error)
}

type SyncServerTransport interface {
	Syncs() <-chan IncomingSyncRequest
	Metadata() map[string]string
}

type SyncingServer struct {
	nginxAdapter *AppStateChangeGenerator

	apps AppSource

	encoder   SyncEncoder
	transport SyncServerTransport
	peers     PeerStorage
}

func (s *SyncingServer) Start() {
	for incomingSync := range s.transport.Syncs() {
		apps, decodeErr := s.encoder.Decode(incomingSync.Request)
		if decodeErr != nil {
			incomingSync.Err <- decodeErr
			continue
		}
		if len(apps) > 0 {
			peer, peerErr := s.peers.GetByName(apps[0].Peer)
			if peerErr != nil {
				incomingSync.Err <- peerErr
				continue
			}
			s.nginxAdapter.OnSync(
				peer.Name,
				apps,
				nil,
			)
		}
		apps, listErr := s.apps.List()
		if listErr != nil {
			incomingSync.Err <- listErr
			continue
		}
		encoded, encodeErr := s.encoder.Encode(apps)
		if encodeErr != nil {
			incomingSync.Err <- encodeErr
			continue
		}
		incomingSync.Response <- encoded
	}
}

func NewSyncingServer(
	nginxAdapter *AppStateChangeGenerator,
	apps AppSource,
	encoder SyncEncoder,
	transport SyncServerTransport,
	peers PeerStorage,
) *SyncingServer {
	return &SyncingServer{
		nginxAdapter: nginxAdapter,
		apps:         apps,
		encoder:      encoder,
		transport:    transport,
		peers:        peers,
	}
}

type SyncingClient struct {
	nginxAdapter *AppStateChangeGenerator
	encoder      SyncEncoder
	interval     time.Duration
	apps         AppSource
	transport    SyncClientTransport
}

func (c *SyncingClient) Start() error {
	for {

		time.Sleep(c.interval)
		apps, listErr := c.apps.List()
		if listErr != nil {
			logrus.Errorf("failed to list apps: %v", listErr)
			continue
		}
		encodedApps, encodeErr := c.encoder.Encode(apps)
		if encodeErr != nil {
			logrus.Errorf("failed to encode apps: %v", encodeErr)
			continue
		}
		incomingApps, err := c.transport.Sync(encodedApps)
		if err != nil {
			logrus.Errorf("failed to sync apps: %v", err)
			continue
		}
		decodedIncomingApps, decodeErr := c.encoder.Decode(incomingApps)
		if decodeErr != nil {
			logrus.Errorf("failed to decode incoming apps: %v", decodeErr)
			continue
		}
		c.nginxAdapter.OnSync(
			"server",
			decodedIncomingApps,
			nil,
		)
	}
}

func NewSyncingClient(
	nginxAdapter *AppStateChangeGenerator,
	encoder SyncEncoder,
	interval time.Duration,
	apps AppSource,
	transport SyncClientTransport,
) *SyncingClient {
	return &SyncingClient{
		nginxAdapter: nginxAdapter,
		encoder:      encoder,
		interval:     interval,
		apps:         apps,
		transport:    transport,
	}
}

type SyncingClientFactory interface {
	New(PairingResponse) (*SyncingClient, error)
}

func NewHTTPSyncingClient(
	nginxAdapter *AppStateChangeGenerator,
	encoder SyncEncoder,
	interval time.Duration,
	apps AppSource,
	pr PairingResponse,

) (*SyncingClient, error) {
	syncServerAddress, ok := pr.Metadata["sync_server_address"]
	if !ok {
		return nil, errors.New("sync_server_address not found in pairing response metadata")
	}
	transport := NewHTTPClientSyncTransport(syncServerAddress)
	return NewSyncingClient(
		nginxAdapter,
		encoder,
		interval,
		apps,
		transport,
	), nil

}

type httpServerSyncTransport struct {
	syncs  chan IncomingSyncRequest
	server *http.Server
}

func (t *httpServerSyncTransport) Syncs() <-chan IncomingSyncRequest {
	return t.syncs
}

func (t *httpServerSyncTransport) Metadata() map[string]string {
	return map[string]string{
		"sync_server_address": fmt.Sprintf("http://%s", t.server.Addr),
	}
}

func NewHTTPServerSyncTransport(server *http.Server) SyncServerTransport {
	syncs := make(chan IncomingSyncRequest)
	router := http.NewServeMux()
	router.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		var req IncomingSyncRequest
		req.Request = make([]byte, r.ContentLength)
		r.Body.Read(req.Request)
		req.Response = make(chan []byte)
		req.Err = make(chan error)
		syncs <- req
		select {
		case resp := <-req.Response:
			w.Write(resp)
		case err := <-req.Err:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	server.Handler = router
	go func() {
		logrus.Infof("Starting HTTP sync transport server on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil {
			logrus.Fatalf("Failed to start HTTP transport server: %v", err)
		}
	}()
	return &httpServerSyncTransport{
		syncs:  syncs,
		server: server,
	}
}

type httpClientSyncTransport struct {
	serverURL string
	client    *http.Client
}

func (t *httpClientSyncTransport) Sync(req []byte) ([]byte, error) {
	resp, err := t.client.Post(t.serverURL+"/sync", "application/octet-stream", bytes.NewReader(req))
	if err != nil {
		return nil, err
	}
	respBody := make([]byte, resp.ContentLength)
	resp.Body.Read(respBody)
	return respBody, nil
}

func NewHTTPClientSyncTransport(serverURL string) SyncClientTransport {
	return &httpClientSyncTransport{
		serverURL: serverURL,
		client:    &http.Client{},
	}
}
