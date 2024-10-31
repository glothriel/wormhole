package pairing

// PairingRequestClientError is an error that indicate, that it's something wrong with the client
type PairingRequestClientError struct {
	Err error
}

func (e PairingRequestClientError) Error() string {
	return e.Err.Error()
}

// NewPairingRequestClientError creates a new PairingRequestClientError instance
func NewPairingRequestClientError(err error) PairingRequestClientError {
	return PairingRequestClientError{Err: err}
}

// PairingRequestServerError is an error that indicate, that client request was OK, but server failed
type PairingRequestServerError struct {
	Err error
}

func (e PairingRequestServerError) Error() string {
	return e.Err.Error()
}

// NewPairingRequestServerError creates a new PairingRequestServerError instance
func NewPairingRequestServerError(err error) PairingRequestServerError {
	return PairingRequestServerError{Err: err}
}

// IncomingPairingRequest is a request that was received by the server
type IncomingPairingRequest struct {
	Request  []byte
	Response chan []byte
	Err      chan error
}

// PairingClientTransport is an interface for sending pairing requests
type PairingClientTransport interface {
	Send([]byte) ([]byte, error)
}

// PairingServerTransport is an interface for receiving pairing requests
type PairingServerTransport interface {
	Requests() <-chan IncomingPairingRequest
}

// IPPool is an interface for managing IP addresses
type IPPool interface {
	Next() (string, error)
}
