package hello

import "encoding/json"

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
