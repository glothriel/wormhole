package syncing

import (
	"encoding/json"

	"github.com/glothriel/wormhole/pkg/apps"
)

// SyncingMessage is a message that contains a list of apps and the peer that sent them
type SyncingMessage struct {
	Peer string
	Apps []apps.App
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
