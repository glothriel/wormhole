package peers

import (
	"github.com/glothriel/wormhole/pkg/messages"
)

// Peer is entity connected to wormhole network, that cen exchange messages with other entities
type Peer interface {
	Name() string
	Send(messages.Message) error
	Receive() (chan messages.Message, error)
	WhenClosed(func())
	AppStatusChanges() chan AppStatus
	Close() error
}

// App is a definition of application exposed by given peer
type App struct {
	Name    string
	Address string
}

const AppStatusAdded = "added"
const AppStatusWithdrawn = "withdrawn"

type AppStatus struct {
	Name string
	App  App
}

// PeerFactory is responsible for creating new peers
type PeerFactory interface {
	Peers() (chan Peer, error)
}
