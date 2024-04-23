package hello

import "github.com/glothriel/wormhole/pkg/peers"

type peerEnrichingAppSource struct {
	peer  string
	child AppSource
}

func (s *peerEnrichingAppSource) List() ([]peers.App, error) {
	theApps, err := s.child.List()
	if err != nil {
		return nil, err
	}
	newApps := make([]peers.App, len(theApps))
	for i := range theApps {
		newApps[i] = peers.WithPeer(theApps[i], s.peer)
	}
	return newApps, nil
}

func NewPeerEnrichingAppSource(peer string, child AppSource) AppSource {
	return &peerEnrichingAppSource{
		peer:  peer,
		child: child,
	}
}
