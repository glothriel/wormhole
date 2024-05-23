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
	peerName       string
	stateGenerator *AppStateChangeGenerator

	apps AppSource

	encoder   SyncingEncoder
	transport SyncServerTransport
	peers     PeerStorage
}

func (s *SyncingServer) Start() {
	for incomingSync := range s.transport.Syncs() {
		msg, decodeErr := s.encoder.Decode(incomingSync.Request)
		if decodeErr != nil {
			incomingSync.Err <- decodeErr
			continue
		}
		peer, peerErr := s.peers.GetByName(msg.Peer)
		if peerErr != nil {
			incomingSync.Err <- peerErr
			continue
		}
		s.stateGenerator.OnSync(
			peer.Name,
			msg.Apps,
			nil,
		)
		apps, listErr := s.apps.List()
		if listErr != nil {
			incomingSync.Err <- listErr
			continue
		}

		encoded, encodeErr := s.encoder.Encode(
			SyncingMessage{
				Peer: s.peerName,
				Apps: apps,
			},
		)
		if encodeErr != nil {
			incomingSync.Err <- encodeErr
			continue
		}
		incomingSync.Response <- encoded
	}
}

func NewSyncingServer(
	myName string,
	stateGenerator *AppStateChangeGenerator,
	apps AppSource,
	encoder SyncingEncoder,
	transport SyncServerTransport,
	peers PeerStorage,
) *SyncingServer {
	return &SyncingServer{
		peerName:       myName,
		stateGenerator: stateGenerator,
		apps:           apps,
		encoder:        encoder,
		transport:      transport,
		peers:          peers,
	}
}

type SyncingClient struct {
	myName       string
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
		encodedApps, encodeErr := c.encoder.Encode(SyncingMessage{
			Peer: c.myName,
			Apps: apps,
		})
		if encodeErr != nil {
			logrus.Errorf("failed to encode apps: %v", encodeErr)
			continue
		}
		incomingApps, err := c.transport.Sync(encodedApps)
		if err != nil {
			logrus.Errorf("failed to sync apps: %v", err)
			continue
		}
		decodedMsg, decodeErr := c.encoder.Decode(incomingApps)
		if decodeErr != nil {
			logrus.Errorf("failed to decode incoming apps: %v", decodeErr)
			continue
		}
		c.nginxAdapter.OnSync(
			decodedMsg.Peer,
			decodedMsg.Apps,
			nil,
		)
	}
}

func NewSyncingClient(
	myName string,
	nginxAdapter *AppStateChangeGenerator,
	encoder SyncingEncoder,
	interval time.Duration,
	apps AppSource,
	transport SyncClientTransport,
) *SyncingClient {
	return &SyncingClient{
		myName:       myName,
		nginxAdapter: nginxAdapter,
		encoder:      encoder,
		interval:     interval,
		apps:         apps,
		transport:    transport,
	}
}

func NewHTTPSyncingClient(
	myName string,
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
		myName,
		nginxAdapter,
		encoder,
		interval,
		apps,
		transport,
	), nil

}
