package hello

import (
	"encoding/json"
	"fmt"

	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

// PairingClient allows pairing with a server
type PairingClient interface {
	Pair() (PairingResponse, error)
}

type keyCachingPairingClient struct {
	client     PairingClient
	storage    KeyCachingPairingClientStorage
	wgConfig   *wg.Config
	wgReloader WireguardConfigReloader

	pinger pinger
}

func (c *keyCachingPairingClient) Pair() (PairingResponse, error) {
	response, getErr := c.storage.Get()
	if getErr == nil {
		c.wgConfig.Address = response.AssignedIP
		c.wgConfig.Upsert(wg.Peer{
			Name:                response.Name,
			Endpoint:            response.Wireguard.Endpoint,
			PublicKey:           response.Wireguard.PublicKey,
			AllowedIPs:          fmt.Sprintf("%s/32,%s/32", response.InternalServerIP, response.AssignedIP),
			PersistentKeepalive: 10,
		})

		updateErr := c.wgReloader.Update(*c.wgConfig)
		if updateErr != nil {
			logrus.Errorf("Failed to update Wireguard config: %v", updateErr)
		}
		logrus.Infof(
			"Trying to ping server %s with the config from the cache", response.InternalServerIP,
		)
		pingerErr := c.pinger.Ping(response.InternalServerIP)
		if pingerErr == nil {
			logrus.Infof("Successfully pinged server %s, using IP from the cache", response.InternalServerIP)
			return response, nil
		}
		logrus.Warnf("Failed to ping server %s: %v, attempting to pair using PSK", response.InternalServerIP, pingerErr)
	} else {
		logrus.Info("No cached pairing response found, pairing with server")
	}
	childResponse, pairErr := c.client.Pair()
	if pairErr != nil {
		return PairingResponse{}, pairErr
	}
	setErr := c.storage.Set(childResponse)
	if setErr != nil {
		logrus.Errorf("Failed to store pairing response: %v", setErr)
	}

	return childResponse, nil
}

// NewKeyCachingPairingClient is a decorator that tries to cache the keys obtained by child client
func NewKeyCachingPairingClient(
	storage KeyCachingPairingClientStorage,
	wgConfig *wg.Config,

	wgReloader WireguardConfigReloader,
	client PairingClient,
) PairingClient {
	return &keyCachingPairingClient{
		client:     client,
		storage:    storage,
		wgReloader: wgReloader,
		wgConfig:   wgConfig,

		pinger: &retryingPinger{&defaultPinger{}},
	}
}

// KeyCachingPairingClientStorage is a storage for pairing responses cache
type KeyCachingPairingClientStorage interface {
	Set(PairingResponse) error
	Get() (PairingResponse, error)
}

type boltKeyCachingPairingClientStorage struct {
	db *bolt.DB
}

func (s *boltKeyCachingPairingClientStorage) Get() (PairingResponse, error) {
	var response PairingResponse
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("pairing"))
		if bucket == nil {
			return fmt.Errorf("bucket does not exist")
		}
		data := bucket.Get([]byte("response"))
		if data == nil {
			return fmt.Errorf("response does not exist")
		}
		return json.Unmarshal(data, &response)
	})
	return response, err
}

func (s *boltKeyCachingPairingClientStorage) Set(response PairingResponse) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket, createErr := tx.CreateBucketIfNotExists([]byte("pairing"))
		if createErr != nil {
			return createErr
		}
		encoded, encodeErr := json.Marshal(response)
		if encodeErr != nil {
			return encodeErr
		}
		return bucket.Put([]byte("response"), encoded)
	})
}

// NewBoltKeyCachingPairingClientStorage creates a new KeyCachingPairingClientStorage backed by a bolt database
func NewBoltKeyCachingPairingClientStorage(path string) (KeyCachingPairingClientStorage, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	return &boltKeyCachingPairingClientStorage{db: db}, nil
}

type inMemoryKeyCachingPairingClientStorage struct {
	isSet    bool
	response PairingResponse
}

func (s *inMemoryKeyCachingPairingClientStorage) Get() (PairingResponse, error) {
	if !s.isSet {
		return PairingResponse{}, fmt.Errorf("response not set")
	}
	return s.response, nil
}

func (s *inMemoryKeyCachingPairingClientStorage) Set(response PairingResponse) error {
	s.response = response
	s.isSet = true
	return nil
}

// NewInMemoryKeyCachingPairingClientStorage creates a new KeyCachingPairingClientStorage backed by memory
func NewInMemoryKeyCachingPairingClientStorage() KeyCachingPairingClientStorage {
	return &inMemoryKeyCachingPairingClientStorage{}
}

// defaultPairingClient is a client that can pair with a server
type defaultPairingClient struct {
	clientName string
	keyPair    KeyPair
	wgConfig   *wg.Config

	wgReloader WireguardConfigReloader
	encoder    PairingEncoder
	transport  PairingClientTransport
}

// Pair sends a pairing request to the server and returns the response
func (c *defaultPairingClient) Pair() (PairingResponse, error) {
	request := PairingRequest{
		Name: c.clientName,
		Wireguard: PairingRequestWireguardConfig{
			PublicKey: c.keyPair.PublicKey,
		},
		Metadata: map[string]string{},
	}
	encoded, encodeErr := c.encoder.EncodeRequest(request)
	if encodeErr != nil {
		return PairingResponse{}, NewPairingRequestClientError(encodeErr)
	}

	response, sendErr := c.transport.Send(encoded)
	if sendErr != nil {
		return PairingResponse{}, NewPairingRequestClientError(sendErr)
	}

	decoded, decodeErr := c.encoder.DecodeResponse(response)
	if decodeErr != nil {
		return PairingResponse{}, NewPairingRequestClientError(decodeErr)
	}
	c.wgConfig.Address = decoded.AssignedIP
	c.wgConfig.Upsert(wg.Peer{
		Name:                decoded.Name,
		Endpoint:            decoded.Wireguard.Endpoint,
		PublicKey:           decoded.Wireguard.PublicKey,
		AllowedIPs:          fmt.Sprintf("%s/32,%s/32", decoded.InternalServerIP, decoded.AssignedIP),
		PersistentKeepalive: 10,
	})

	return decoded, c.wgReloader.Update(*c.wgConfig)
}

// NewDefaultPairingClient executes HTTP pairing requests to the server
func NewDefaultPairingClient(
	clientName string,
	wgConfig *wg.Config,
	keyPair KeyPair,
	wgReloader WireguardConfigReloader,
	encoder PairingEncoder,
	transport PairingClientTransport,
) PairingClient {
	return &defaultPairingClient{
		clientName: clientName,
		keyPair:    keyPair,
		wgConfig:   wgConfig,
		wgReloader: wgReloader,
		encoder:    encoder,
		transport:  transport,
	}
}

// MetadataEnricher is an interface that allows transports exchanging information between
// their client/server implementations
type MetadataEnricher interface {
	Metadata() map[string]string
}

// PairingServer is a server that can pair with multiple clients
type PairingServer struct {
	serverName       string     // Name of the server peer
	publicWgHostPort string     // Public Wireguard host:port
	wgConfig         *wg.Config // Local Wireguard config
	keyPair          KeyPair    // Local Wireguard key pair

	wgReloader WireguardConfigReloader
	marshaler  PairingEncoder
	transport  PairingServerTransport
	ips        IPPool
	storage    PeerStorage
	enrichers  []MetadataEnricher
}

// Start starts the pairing server
func (s *PairingServer) Start() { // nolint: funlen, gocognit
	for incomingRequest := range s.transport.Requests() {
		request, requestErr := s.marshaler.DecodeRequest(incomingRequest.Request)
		if requestErr != nil {
			incomingRequest.Err <- NewPairingRequestClientError(requestErr)
			continue
		}

		var ip string
		var publicKey string
		existingPeer, peerErr := s.storage.GetByName(request.Name)
		if peerErr != nil {
			if peerErr != ErrPeerDoesNotExist {
				incomingRequest.Err <- NewPairingRequestServerError(peerErr)
				continue
			}
			// Peer is not in the Database
			var ipErr error
			ip, ipErr = s.ips.Next()
			if ipErr != nil {
				incomingRequest.Err <- NewPairingRequestServerError(ipErr)
				continue
			}
			publicKey = request.Wireguard.PublicKey

			// Store peer info
			storeErr := s.storage.Store(PeerInfo{
				Name:      request.Name,
				IP:        ip,
				PublicKey: publicKey,
			})

			if storeErr != nil {
				incomingRequest.Err <- NewPairingRequestServerError(storeErr)
				continue
			}
		} else {
			if existingPeer.PublicKey != request.Wireguard.PublicKey {
				logrus.Errorf(
					"attempted peering from peer %s: error, public key mismatch", request.Name,
				)
				continue
			}
			// Peer is in the Database
			ip = existingPeer.IP
			publicKey = existingPeer.PublicKey
		}

		// Update local wireguard config
		s.wgConfig.Upsert(wg.Peer{
			Name:       request.Name,
			PublicKey:  publicKey,
			AllowedIPs: fmt.Sprintf("%s/32,%s/32", ip, s.wgConfig.Address),
		})
		wgUpdateErr := s.wgReloader.Update(*s.wgConfig)
		if wgUpdateErr != nil {
			incomingRequest.Err <- NewPairingRequestServerError(wgUpdateErr)
			continue
		}

		// Enrich metadata
		metadata := map[string]string{}
		for _, enricher := range s.enrichers {
			for k, v := range enricher.Metadata() {
				metadata[k] = v
			}
		}

		// Respond to the client
		response := PairingResponse{
			Name:             s.serverName,
			AssignedIP:       ip,
			InternalServerIP: s.wgConfig.Address,
			Wireguard: PairingResponseWireguardConfig{
				PublicKey: s.keyPair.PublicKey,
				Endpoint:  s.publicWgHostPort,
			},
			Metadata: metadata,
		}
		encoded, encodeErr := s.marshaler.EncodeResponse(response)
		if encodeErr != nil {
			incomingRequest.Err <- NewPairingRequestServerError(encodeErr)
			continue
		}
		logrus.Infof("Pairing request from %s, assigned IP %s", request.Name, response.AssignedIP)
		incomingRequest.Response <- encoded
	}
}

// NewPairingServer creates a new PairingServer instance
func NewPairingServer(
	serverName string,
	publicWgHostPort string,
	wgConfig *wg.Config,
	keyPair KeyPair,
	wgReloader WireguardConfigReloader,
	encoder PairingEncoder,
	transport PairingServerTransport,
	ips IPPool,
	storage PeerStorage,
	enrichers []MetadataEnricher,
) *PairingServer {
	return &PairingServer{
		serverName:       serverName,
		publicWgHostPort: publicWgHostPort,
		wgConfig:         wgConfig,
		keyPair:          keyPair,
		wgReloader:       wgReloader,
		marshaler:        encoder,
		transport:        transport,
		ips:              ips,
		storage:          storage,
		enrichers:        enrichers,
	}
}
