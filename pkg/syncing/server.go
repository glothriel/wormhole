package syncing

import (
	"github.com/glothriel/wormhole/pkg/pairing"
)

// IncomingSyncRequest is a struct that represents raw incoming sync requests
type IncomingSyncRequest struct {
	Request  []byte
	Response chan []byte
	Err      chan error
}

// ClientTransport is an interface for syncing clients transport. Example implementations
// can be http, grpc, etc.
type ClientTransport interface {
	Sync([]byte) ([]byte, error)
}

// ServerTransport is an interface for syncing servers transport, similar to SyncClientTransport
type ServerTransport interface {
	Syncs() <-chan IncomingSyncRequest
	Metadata() map[string]string
}

// Server orchestrates all the operations that are performed server-side
// when executing app list synchronizations
type Server struct {
	myName         string
	stateGenerator *AppStateChangeGenerator

	apps AppSource

	encoder   Encoder
	transport ServerTransport
	peers     pairing.PeerStorage
}

// Start starts the syncing server
func (s *Server) Start() {
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
		s.stateGenerator.UpdateForPeer(
			peer.Name,
			msg.Apps,
		)
		apps, listErr := s.apps.List()
		if listErr != nil {
			incomingSync.Err <- listErr
			continue
		}

		encoded, encodeErr := s.encoder.Encode(
			Message{
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

// NewServer creates a new SyncingServer instance
func NewServer(
	myName string,
	stateGenerator *AppStateChangeGenerator,
	apps AppSource,
	encoder Encoder,
	transport ServerTransport,
	peers pairing.PeerStorage,
) *Server {
	return &Server{
		myName:         myName,
		stateGenerator: stateGenerator,
		apps:           apps,
		encoder:        encoder,
		transport:      transport,
		peers:          peers,
	}
}
