package pairing

import (
	"encoding/json"
)

// Encoder is an interface for encoding and decoding pairing requests and responses
type Encoder interface {
	EncodeRequest(Request) ([]byte, error)
	DecodeRequest([]byte) (Request, error)

	EncodeResponse(Response) ([]byte, error)
	DecodeResponse([]byte) (Response, error)
}

type jsonPairingEncoder struct{}

func (e *jsonPairingEncoder) EncodeRequest(req Request) ([]byte, error) {
	return json.Marshal(req)
}

func (e *jsonPairingEncoder) DecodeRequest(data []byte) (Request, error) {
	var req Request
	err := json.Unmarshal(data, &req)
	return req, err
}

func (e *jsonPairingEncoder) EncodeResponse(resp Response) ([]byte, error) {
	return json.Marshal(resp)
}

func (e *jsonPairingEncoder) DecodeResponse(data []byte) (Response, error) {
	var resp Response
	err := json.Unmarshal(data, &resp)
	return resp, err
}

// NewJSONPairingEncoder creates a new PairingEncoder instance
func NewJSONPairingEncoder() Encoder {
	return &jsonPairingEncoder{}
}
