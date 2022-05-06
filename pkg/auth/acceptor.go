package auth

import "crypto/rsa"

// DummyAcceptor implements Acceptor by blindly trusting all keys
type DummyAcceptor struct {
}

// IsTrusted implements Acceptor
func (a DummyAcceptor) IsTrusted(*rsa.PublicKey) (bool, error) {
	return true, nil
}
