package syncing

import (
	"encoding/json"

	"github.com/glothriel/wormhole/pkg/apps"
)

// Message is a message that contains a list of apps and the peer that sent them
type Message struct {
	Peer     string
	Metadata Metadata
	Apps     []apps.App
}

// Encoder is an interface for encoding and decoding syncing messages
type Encoder interface {
	Encode(Message) ([]byte, error)
	Decode([]byte) (Message, error)
}

type jsonEncoder struct{}

func (e *jsonEncoder) Encode(apps Message) ([]byte, error) {
	return json.Marshal(apps)
}

func (e *jsonEncoder) Decode(data []byte) (Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	return msg, err
}

// NewJSONSyncingEncoder creates a new SyncingEncoder instance
func NewJSONSyncingEncoder() Encoder {
	return &jsonEncoder{}
}
