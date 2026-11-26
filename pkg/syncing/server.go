package syncing

import (
	"sync"

	"github.com/glothriel/wormhole/pkg/pairing"
	"github.com/sirupsen/logrus"
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
	metadata  MetadataStorage

	lock sync.Mutex
}

// Start starts the syncing server
func (s *Server) Start() {
	for incomingSync := range s.transport.Syncs() {
		response, err := s.handleSync(incomingSync.Request)
		if err != nil {
			incomingSync.Err <- err
			continue
		}
		incomingSync.Response <- response
	}
}

// handleSync handles a single sync request, never interleaved with DeletePeer
func (s *Server) handleSync(request []byte) ([]byte, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	msg, decodeErr := s.encoder.Decode(request)
	if decodeErr != nil {
		return nil, decodeErr
	}

	peer, peerErr := s.peers.GetByName(msg.Peer)
	if peerErr != nil {
		return nil, peerErr
	}
	metadataSetErr := s.metadata.Set(peer.Name, msg.Metadata)
	if metadataSetErr != nil {
		return nil, metadataSetErr
	}
	s.stateGenerator.UpdateForPeer(
		peer.Name,
		msg.Apps,
	)
	apps, listErr := s.apps.List()
	if listErr != nil {
		return nil, listErr
	}

	return s.encoder.Encode(
		Message{
			Peer: s.myName,
			Apps: apps,
		},
	)
}

// DeletePeer calls deleteRecord to delete the peer record and, if that succeeds, removes the peer's
// metadata and withdraws its apps. Syncs are not handled while DeletePeer runs.
func (s *Server) DeletePeer(peer string, deleteRecord func() error) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if err := deleteRecord(); err != nil {
		return err
	}
	if err := s.metadata.Delete(peer); err != nil {
		logrus.Errorf("Could not delete metadata of peer %s: %v", peer, err)
	}
	s.stateGenerator.RemovePeer(peer)
	return nil
}

// NewServer creates a new SyncingServer instance
func NewServer(
	myName string,
	stateGenerator *AppStateChangeGenerator,
	apps AppSource,
	encoder Encoder,
	transport ServerTransport,
	peers pairing.PeerStorage,
	metadata MetadataStorage,
) *Server {
	return &Server{
		myName:         myName,
		stateGenerator: stateGenerator,
		apps:           apps,
		encoder:        encoder,
		transport:      transport,
		peers:          peers,
		metadata:       metadata,
	}
}
