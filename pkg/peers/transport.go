package peers

import (
	"github.com/glothriel/wormhole/pkg/messages"
)

// Transport is used to allow communication between the nodes
type Transport interface {
	Send(messages.Message) error
	Receive() (chan messages.Message, error)
	Close() error
}

// TransportFactory creates Transport instances
type TransportFactory interface {
	Create() (Transport, error)
}

// MockTransport implements Transport and can be used for unit tests
type MockTransport struct {
	theOtherOne *MockTransport

	inbox chan messages.Message

	closed bool
}

// Send implements Transport
func (transport *MockTransport) Send(message messages.Message) error {
	transport.theOtherOne.inbox <- message
	return nil
}

// Receive implements Transport
func (transport *MockTransport) Receive() (chan messages.Message, error) {
	return transport.inbox, nil
}

// Close implements Transport
func (transport *MockTransport) Close() error {
	transport.closed = true
	close(transport.inbox)
	return nil
}

// CreateMockTransportPair creates two mock transports
func CreateMockTransportPair() (*MockTransport, *MockTransport) {
	first := &MockTransport{
		inbox: make(chan messages.Message, 255),
	}
	second := &MockTransport{
		inbox: make(chan messages.Message, 255),

		theOtherOne: first,
	}
	first.theOtherOne = second
	return first, second
}
