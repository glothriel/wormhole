package pairing

// KeyPair is a pair of public and private keys
type KeyPair struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

// PeerInfo is a struct that contains information about a peer
type PeerInfo struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	PublicKey string `json:"public_key"`
}
