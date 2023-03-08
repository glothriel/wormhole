package peers

import (
	"github.com/glothriel/wormhole/pkg/messages"
)

// MockPeer implements Peer and can be used for unit tests
type MockPeer struct {
	callbacks []func()

	// AppEventsPeer can be used to force the mock to emit AppEvents
	AppEventsPeer chan AppEvent

	// MessaesFromPeer can be used to simulate, that the mock emits messages
	MessagesFromPeer chan messages.Message

	// MessagesToPeer are used to simulate sending messages to remote peer
	MessagesToPeer chan messages.Message

	// SessionEvents can be used to simulate, that the mock emits session changing events
	MessagesSessionEvents chan messages.Message
}

// Send implements Peer
func (wt *MockPeer) Send(message messages.Message) error {
	wt.MessagesToPeer <- message
	return nil
}

// Close implements Peer
func (wt *MockPeer) Close() error {
	for _, cb := range wt.callbacks {
		cb()
	}
	close(wt.MessagesFromPeer)
	return nil
}

// Name implements Peer
func (wt *MockPeer) Name() string {
	return "mock"
}

// Packets implements Peer
func (wt *MockPeer) Packets() chan messages.Message {
	return wt.MessagesFromPeer
}

// SessionEvents implements Peer
func (wt *MockPeer) SessionEvents() chan messages.Message {
	return wt.MessagesFromPeer
}

// AppEvents implements Peer
func (wt *MockPeer) AppEvents() chan AppEvent {
	return wt.AppEventsPeer
}

// NewMockPeer creates MockPeer instances
func NewMockPeer() *MockPeer {
	return &MockPeer{
		MessagesToPeer:   make(chan messages.Message, 1024),
		MessagesFromPeer: make(chan messages.Message),
		AppEventsPeer:    make(chan AppEvent),
	}
}

type mockPerFactory struct {
	peers chan Peer
}

func (mockFactory *mockPerFactory) Peers() (chan Peer, error) {
	return mockFactory.peers, nil
}

// NewMockPeerFactory creates mockPeerFactory instances
func NewMockPeerFactory(peers chan Peer) PeerFactory {
	return &mockPerFactory{peers: peers}
}
