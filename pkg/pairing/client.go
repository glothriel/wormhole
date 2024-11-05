// Package pairing provides client-server pairing functionality
package pairing

import (
	"encoding/json"
	"fmt"

	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

// Request is a request to pair with a server
type Request struct {
	Name string `json:"name"` // Name of the peer, that requests pairing,
	//  for example `dev1`, `us-east-1`, etc
	Wireguard RequestWireguardConfig `json:"wireguard"`
	Metadata  map[string]string      `json:"metadata"` // Any protocol-specific metadata
}

// RequestWireguardConfig is a wireguard configuration for the pairing request
type RequestWireguardConfig struct {
	PublicKey string `json:"public_key"`
}

// Response is a response to a pairing request
type Response struct {
	Name       string `json:"name"`        // Name of the server peer
	AssignedIP string `json:"assigned_ip"` // IP that the server assigned to the peer,
	// that requested pairing
	InternalServerIP string                  `json:"internal_server_ip"` // IP of the server in the internal network
	Wireguard        ResponseWireguardConfig `json:"wireguard"`
	Metadata         map[string]string       `json:"metadata"` // Any protocol-specific metadata
}

// ResponseWireguardConfig is a wireguard configuration for the pairing response
type ResponseWireguardConfig struct {
	PublicKey string `json:"public_key"`
	Endpoint  string `json:"endpoint"`
}

// Client allows pairing with a server
type Client interface {
	Pair() (Response, error)
}

type keyCachingPairingClient struct {
	client     Client
	storage    KeyCachingPairingClientStorage
	wgConfig   *wg.Config
	wgReloader wg.WireguardConfigReloader

	pinger pinger
}

func (c *keyCachingPairingClient) Pair() (Response, error) {
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
		return Response{}, pairErr
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

	wgReloader wg.WireguardConfigReloader,
	client Client,
) Client {
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
	Set(Response) error
	Get() (Response, error)
}

type boltKeyCachingPairingClientStorage struct {
	db *bolt.DB
}

func (s *boltKeyCachingPairingClientStorage) Get() (Response, error) {
	var response Response
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

func (s *boltKeyCachingPairingClientStorage) Set(response Response) error {
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
	response Response
}

func (s *inMemoryKeyCachingPairingClientStorage) Get() (Response, error) {
	if !s.isSet {
		return Response{}, fmt.Errorf("response not set")
	}
	return s.response, nil
}

func (s *inMemoryKeyCachingPairingClientStorage) Set(response Response) error {
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

	wgReloader wg.WireguardConfigReloader
	encoder    Encoder
	transport  ClientTransport
}

// Pair sends a pairing request to the server and returns the response
func (c *defaultPairingClient) Pair() (Response, error) {
	request := Request{
		Name: c.clientName,
		Wireguard: RequestWireguardConfig{
			PublicKey: c.keyPair.PublicKey,
		},
		Metadata: map[string]string{},
	}
	encoded, encodeErr := c.encoder.EncodeRequest(request)
	if encodeErr != nil {
		return Response{}, NewClientError(encodeErr)
	}

	response, sendErr := c.transport.Send(encoded)
	if sendErr != nil {
		return Response{}, NewClientError(sendErr)
	}

	decoded, decodeErr := c.encoder.DecodeResponse(response)
	if decodeErr != nil {
		return Response{}, NewClientError(decodeErr)
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
	wgReloader wg.WireguardConfigReloader,
	encoder Encoder,
	transport ClientTransport,
) Client {
	return &defaultPairingClient{
		clientName: clientName,
		keyPair:    keyPair,
		wgConfig:   wgConfig,
		wgReloader: wgReloader,
		encoder:    encoder,
		transport:  transport,
	}
}
