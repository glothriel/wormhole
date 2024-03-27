package svcdetector

type PeerDetector interface {
	Peer() string
}

type staticPeerDetector struct {
	peer string
}

func (detector *staticPeerDetector) Peer() string {
	return detector.peer
}

// NewStaticPeerDetector creates a new static peer detector
func NewStaticPeerDetector(peer string) PeerDetector {
	return &staticPeerDetector{
		peer: peer,
	}
}
