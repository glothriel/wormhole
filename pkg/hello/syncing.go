package hello

import (
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// IncomingSyncRequest is a struct that represents raw incoming sync requests
type IncomingSyncRequest struct {
	Request  []byte
	Response chan []byte
	Err      chan error
}

// SyncClientTransport is an interface for syncing clients transport. Example implementations
// can be http, grpc, etc.
type SyncClientTransport interface {
	Sync([]byte) ([]byte, error)
}

// SyncServerTransport is an interface for syncing servers transport, similar to SyncClientTransport
type SyncServerTransport interface {
	Syncs() <-chan IncomingSyncRequest
	Metadata() map[string]string
}

// SyncingServer orchestrates all the operations that are performed server-side
// when executing app list synchronizations
type SyncingServer struct {
	myName         string
	stateGenerator *AppStateChangeGenerator

	apps AppSource

	encoder   SyncingEncoder
	transport SyncServerTransport
	peers     PeerStorage
}

// Start starts the syncing server
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
		s.stateGenerator.SetState(
			peer.Name,
			msg.Apps,
		)
		apps, listErr := s.apps.List()
		if listErr != nil {
			incomingSync.Err <- listErr
			continue
		}

		encoded, encodeErr := s.encoder.Encode(
			SyncingMessage{
				Peer: s.myName,
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

// NewSyncingServer creates a new SyncingServer instance
func NewSyncingServer(
	myName string,
	stateGenerator *AppStateChangeGenerator,
	apps AppSource,
	encoder SyncingEncoder,
	transport SyncServerTransport,
	peers PeerStorage,
) *SyncingServer {
	return &SyncingServer{
		myName:         myName,
		stateGenerator: stateGenerator,
		apps:           apps,
		encoder:        encoder,
		transport:      transport,
		peers:          peers,
	}
}

// SyncingClient is a struct that orchestrates all the operations that are performed client-side
// when executing app list synchronizations
type SyncingClient struct {
	myName               string
	stateChangeGenerator *AppStateChangeGenerator
	encoder              SyncingEncoder
	interval             time.Duration
	apps                 AppSource
	transport            SyncClientTransport
	failureThreshold     int
}

// Start starts the syncing client
func (c *SyncingClient) Start() error {
	failures := 0
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
			if failures >= c.failureThreshold {
				return fmt.Errorf("Fatal: failed to sync %d times in a row: %v", failures, err)
			}
			failures++
			logrus.Errorf("failed to sync apps: %v", err)
			continue
		}
		failures = 0
		decodedMsg, decodeErr := c.encoder.Decode(incomingApps)
		if decodeErr != nil {
			logrus.Errorf("failed to decode incoming apps: %v", decodeErr)
			continue
		}
		c.stateChangeGenerator.SetState(
			decodedMsg.Peer,
			decodedMsg.Apps,
		)
	}
}

// NewSyncingClient creates a new SyncingClient instance
func NewSyncingClient(
	myName string,
	nginxAdapter *AppStateChangeGenerator,
	encoder SyncingEncoder,
	interval time.Duration,
	apps AppSource,
	transport SyncClientTransport,
) *SyncingClient {
	return &SyncingClient{
		myName:               myName,
		stateChangeGenerator: nginxAdapter,
		encoder:              encoder,
		interval:             interval,
		apps:                 apps,
		transport:            transport,
		failureThreshold:     3,
	}
}

// NewHTTPSyncingClient creates a new SyncingClient instance with HTTP transport
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
	transport := NewHTTPClientSyncingTransport(syncServerAddress, 3*time.Second)
	return NewSyncingClient(
		myName,
		nginxAdapter,
		encoder,
		interval,
		apps,
		transport,
	), nil
}
