package hello

import (
	"encoding/json"

	"github.com/glothriel/wormhole/pkg/peers"
)

type Marshaler interface {
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

func NewJSONPairingEncoder() Marshaler {
	return &jsonPairingEncoder{}
}

type SyncingMessage struct {
	Peer string
	Apps []peers.App
}

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

func NewJSONSyncEncoder() SyncingEncoder {
	return &jsonSyncingEncoder{}
}
