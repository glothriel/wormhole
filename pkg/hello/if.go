package hello

import (
	"fmt"

	"github.com/glothriel/wormhole/pkg/wg"
)

type KeyPair struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

type PairingRequest struct {
	Name      string                        `json:"name"` // Name of the peer, that requests pairing, for example `dev1`, `us-east-1`, etc
	Wireguard PairingRequestWireguardConfig `json:"wireguard"`
	Metadata  map[string]string             `json:"metadata"` // Any protocol-specific metadata
}

type PairingRequestWireguardConfig struct {
	PublicKey string `json:"public_key"`
}

type PairingResponse struct {
	Name             string                         `json:"name"`               // Name of the server peer
	AssignedIP       string                         `json:"assigned_ip"`        // IP that the server assigned to the peer, that requested pairing
	InternalServerIP string                         `json:"internal_server_ip"` // IP of the server in the internal network
	Wireguard        PairingResponseWireguardConfig `json:"wireguard"`
	Metadata         map[string]string              `json:"metadata"` // Any protocol-specific metadata
}

type PairingResponseWireguardConfig struct {
	PublicKey string `json:"public_key"`
	Endpoint  string `json:"endpoint"`
}

type IPPool interface {
	// TODO: This interface is not complete, it should have at least a method to release IP
	Next() (string, error)
}

type PairingEncoder interface {
	EncodeRequest(PairingRequest) ([]byte, error)
	DecodeRequest([]byte) (PairingRequest, error)

	EncodeResponse(PairingResponse) ([]byte, error)
	DecodeResponse([]byte) (PairingResponse, error)
}

type PairingRequestClientError struct {
	Err error
}

func (e PairingRequestClientError) Error() string {
	return e.Err.Error()
}

func NewPairingRequestClientError(err error) PairingRequestClientError {
	return PairingRequestClientError{Err: err}
}

type PairingRequestServerError struct {
	Err error
}

func (e PairingRequestServerError) Error() string {
	return e.Err.Error()
}

func NewPairingRequestServerError(err error) PairingRequestServerError {
	return PairingRequestServerError{Err: err}
}

type IncomingPairingRequest struct {
	Request  []byte
	Response chan []byte
	Err      chan error
}

type PairingClientTransport interface {
	Send([]byte) ([]byte, error)
}

type PairingServerTransport interface {
	Requests() <-chan IncomingPairingRequest
}

type PairingTransport interface {
	Client(KeyPair) PairingClientTransport
	Server(KeyPair) PairingServerTransport
}

type WireguardConfigReloader interface {
	Update(wg.Config) error
}

type PeerInfo struct {
	Name        string `json:"name"`
	IP          string `json:"ip"`
	PublicKey   string `json:"public_key"`
	LastContact int64  `json:"last_contact"`
}

type PeerStorage interface {
	Store(PeerInfo) error
	GetByName(string) (PeerInfo, error)
	GetByIP(string) (PeerInfo, error)
	List() ([]PeerInfo, error)
}

type PairingServer struct {
	serverName       string     // Name of the server peer
	publicWgHostPort string     // Public Wireguard host:port, used in Endpoint field of the Wireguard config of other peers
	wgConfig         *wg.Config // Local Wireguard config
	keyPair          KeyPair    // Local Wireguard key pair

	wgReloader WireguardConfigReloader
	encoder    PairingEncoder
	transport  PairingServerTransport
	ips        IPPool
	storage    PeerStorage
}

func (s *PairingServer) Start() {
	for incomingRequest := range s.transport.Requests() {
		request, requestErr := s.encoder.DecodeRequest(incomingRequest.Request)
		if requestErr != nil {
			incomingRequest.Err <- NewPairingRequestClientError(requestErr)
			continue
		}

		// Assign IP
		ip, ipErr := s.ips.Next()
		if ipErr != nil {
			incomingRequest.Err <- NewPairingRequestServerError(ipErr)
			continue
		}

		// Update local wireguard config
		s.wgConfig.Upsert(wg.Peer{
			PublicKey:  request.Wireguard.PublicKey,
			AllowedIPs: fmt.Sprintf("%s/32,%s/32", ip, s.wgConfig.Address),
		})
		s.wgReloader.Update(*s.wgConfig)

		// Store peer info
		storeErr := s.storage.Store(PeerInfo{
			Name:      request.Name,
			IP:        ip,
			PublicKey: request.Wireguard.PublicKey,
		})
		if storeErr != nil {
			incomingRequest.Err <- NewPairingRequestServerError(storeErr)
			continue
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
			Metadata: map[string]string{},
		}
		encoded, encodeErr := s.encoder.EncodeResponse(response)
		if encodeErr != nil {
			incomingRequest.Err <- NewPairingRequestServerError(encodeErr)
			continue
		}
		incomingRequest.Response <- encoded
	}
}

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
) *PairingServer {
	return &PairingServer{
		serverName:       serverName,
		publicWgHostPort: publicWgHostPort,
		wgConfig:         wgConfig,
		keyPair:          keyPair,
		wgReloader:       wgReloader,
		encoder:          encoder,
		transport:        transport,
		ips:              ips,
		storage:          storage,
	}
}
