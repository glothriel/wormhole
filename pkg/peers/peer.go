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
	AppEvents() chan AppEvent
	Close() error
}

// App is a definition of application exposed by given peer
type App struct {
	Name    string
	Address string
}

// EventAppAdded is emmited, when peer wants to expose a new app
const EventAppAdded = messages.TypeAppAdded

// EventAppWithdrawn is emmited, when peer no longer wants to expose an app
const EventAppWithdrawn = messages.TypeAppWithdrawn

// AppEvent is a change in app exposure status
type AppEvent struct {
	Type string
	App  App
}

// PeerFactory is responsible for creating new peers
type PeerFactory interface {
	Peers() (chan Peer, error)
}
