package hello

import (
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
