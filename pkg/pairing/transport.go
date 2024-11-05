package pairing

// ClientTransport is an interface for sending pairing requests
type ClientTransport interface {
	Send([]byte) ([]byte, error)
}

// ServerTransport is an interface for receiving pairing requests
type ServerTransport interface {
	Requests() <-chan IncomingPairingRequest
}

// IncomingPairingRequest is a request that was received by the server
type IncomingPairingRequest struct {
	Request  []byte
	Response chan []byte
	Err      chan error
}
