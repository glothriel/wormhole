package hello

import (
	"encoding/json"

	"github.com/glothriel/wormhole/pkg/peers"
)

// PairingEncoder is an interface for encoding and decoding pairing requests and responses
type PairingEncoder interface {
	EncodeRequest(PairingRequest) ([]byte, error)
	DecodeRequest([]byte) (PairingRequest, error)

	EncodeResponse(PairingResponse) ([]byte, error)
	DecodeResponse([]byte) (PairingResponse, error)
}

type jsonPairingEncoder struct{}

func (e *jsonPairingEncoder) EncodeRequest(req PairingRequest) ([]byte, error) {
	return json.Marshal(req)
}

func (e *jsonPairingEncoder) DecodeRequest(data []byte) (PairingRequest, error) {
	var req PairingRequest
	err := json.Unmarshal(data, &req)
	return req, err
}

func (e *jsonPairingEncoder) EncodeResponse(resp PairingResponse) ([]byte, error) {
	return json.Marshal(resp)
}

func (e *jsonPairingEncoder) DecodeResponse(data []byte) (PairingResponse, error) {
	var resp PairingResponse
	err := json.Unmarshal(data, &resp)
	return resp, err
}

// NewJSONPairingEncoder creates a new PairingEncoder instance
func NewJSONPairingEncoder() PairingEncoder {
	return &jsonPairingEncoder{}
}

// SyncingMessage is a message that contains a list of apps and the peer that sent them
type SyncingMessage struct {
	Peer string
	Apps []peers.App
}

// SyncingEncoder is an interface for encoding and decoding syncing messages
type SyncingEncoder interface {
	Encode(SyncingMessage) ([]byte, error)
	Decode([]byte) (SyncingMessage, error)
}

type jsonSyncingEncoder struct{}

func (e *jsonSyncingEncoder) Encode(apps SyncingMessage) ([]byte, error) {
	return json.Marshal(apps)
}

func (e *jsonSyncingEncoder) Decode(data []byte) (SyncingMessage, error) {
	var msg SyncingMessage
	err := json.Unmarshal(data, &msg)
	return msg, err
}

// NewJSONSyncingEncoder creates a new SyncingEncoder instance
func NewJSONSyncingEncoder() SyncingEncoder {
	return &jsonSyncingEncoder{}
}
