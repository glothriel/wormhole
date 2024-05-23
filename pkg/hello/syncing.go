package hello

import (
	"errors"
	"time"

	"github.com/sirupsen/logrus"
)

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
	stateGenerator *AppStateChangeGenerator

	apps AppSource

	encoder   SyncingEncoder
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
			s.stateGenerator.OnSync(
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
	stateGenerator *AppStateChangeGenerator,
	apps AppSource,
	encoder SyncingEncoder,
	transport SyncServerTransport,
	peers PeerStorage,
) *SyncingServer {
	return &SyncingServer{
		stateGenerator: stateGenerator,
		apps:           apps,
		encoder:        encoder,
		transport:      transport,
		peers:          peers,
	}
}

type SyncingClient struct {
	nginxAdapter *AppStateChangeGenerator
	encoder      SyncingEncoder
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
	encoder SyncingEncoder,
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

func NewHTTPSyncingClient(
	nginxAdapter *AppStateChangeGenerator,
	encoder SyncingEncoder,
	interval time.Duration,
	apps AppSource,
	pr PairingResponse,

) (*SyncingClient, error) {
	syncServerAddress, ok := pr.Metadata["sync_server_address"]
	if !ok {
		return nil, errors.New("sync_server_address not found in pairing response metadata")
	}
	transport := NewHTTPClientSyncingTransport(syncServerAddress)
	return NewSyncingClient(
		nginxAdapter,
		encoder,
		interval,
		apps,
		transport,
	), nil

}
