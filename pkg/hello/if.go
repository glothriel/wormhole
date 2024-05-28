package hello

import (
	"github.com/glothriel/wormhole/pkg/wg"
)

// KeyPair is a pair of public and private keys
type KeyPair struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

// PairingRequest is a request to pair with a server
type PairingRequest struct {
	Name string `json:"name"` // Name of the peer, that requests pairing,
	//  for example `dev1`, `us-east-1`, etc
	Wireguard PairingRequestWireguardConfig `json:"wireguard"`
	Metadata  map[string]string             `json:"metadata"` // Any protocol-specific metadata
}

// PairingRequestWireguardConfig is a wireguard configuration for the pairing request
type PairingRequestWireguardConfig struct {
	PublicKey string `json:"public_key"`
}

// PairingResponse is a response to a pairing request
type PairingResponse struct {
	Name       string `json:"name"`        // Name of the server peer
	AssignedIP string `json:"assigned_ip"` // IP that the server assigned to the peer,
	// that requested pairing
	InternalServerIP string                         `json:"internal_server_ip"` // IP of the server in the internal network
	Wireguard        PairingResponseWireguardConfig `json:"wireguard"`
	Metadata         map[string]string              `json:"metadata"` // Any protocol-specific metadata
}

// PairingResponseWireguardConfig is a wireguard configuration for the pairing response
type PairingResponseWireguardConfig struct {
	PublicKey string `json:"public_key"`
	Endpoint  string `json:"endpoint"`
}

// IPPool is an interface for managing IP addresses
type IPPool interface {
	Next() (string, error)
}

// PairingRequestClientError is an error that indicate, that it's something wrong with the client
type PairingRequestClientError struct {
	Err error
}

func (e PairingRequestClientError) Error() string {
	return e.Err.Error()
}

// NewPairingRequestClientError creates a new PairingRequestClientError instance
func NewPairingRequestClientError(err error) PairingRequestClientError {
	return PairingRequestClientError{Err: err}
}

// PairingRequestServerError is an error that indicate, that client request was OK, but server failed
type PairingRequestServerError struct {
	Err error
}

func (e PairingRequestServerError) Error() string {
	return e.Err.Error()
}

// NewPairingRequestServerError creates a new PairingRequestServerError instance
func NewPairingRequestServerError(err error) PairingRequestServerError {
	return PairingRequestServerError{Err: err}
}

// IncomingPairingRequest is a request that was received by the server
type IncomingPairingRequest struct {
	Request  []byte
	Response chan []byte
	Err      chan error
}

// PairingClientTransport is an interface for sending pairing requests
type PairingClientTransport interface {
	Send([]byte) ([]byte, error)
}

// PairingServerTransport is an interface for receiving pairing requests
type PairingServerTransport interface {
	Requests() <-chan IncomingPairingRequest
}

// WireguardConfigReloader is an interface for updating Wireguard configuration
type WireguardConfigReloader interface {
	Update(wg.Config) error
}

// PeerInfo is a struct that contains information about a peer
type PeerInfo struct {
	Name        string `json:"name"`
	IP          string `json:"ip"`
	PublicKey   string `json:"public_key"`
	LastContact int64  `json:"last_contact"`
}
