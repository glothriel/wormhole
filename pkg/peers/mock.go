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

// Receive implements Peer
func (wt *MockPeer) Receive() (chan messages.Message, error) {
	return wt.MessagesFromPeer, nil
}

// AppEvents implements Peer
func (wt *MockPeer) AppEvents() chan AppEvent {
	return wt.AppEventsPeer
}

// WhenClosed implements Peer
func (wt *MockPeer) WhenClosed(cb func()) {
	wt.callbacks = append(wt.callbacks, cb)
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
