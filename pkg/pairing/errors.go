package pairing

// ClientError is an error that indicate, that it's something wrong with the client
type ClientError struct {
	Err error
}

func (e ClientError) Error() string {
	return e.Err.Error()
}

// NewClientError creates a new PairingRequestClientError instance
func NewClientError(err error) ClientError {
	return ClientError{Err: err}
}

// ServerError is an error that indicate, that client request was OK, but server failed
type ServerError struct {
	Err error
}

func (e ServerError) Error() string {
	return e.Err.Error()
}

// NewServerError creates a new PairingRequestServerError instance
func NewServerError(err error) ServerError {
	return ServerError{Err: err}
}
