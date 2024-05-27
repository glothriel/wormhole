// Package hello provides the protocol between the client and the server.
// Ultimately it should be split into two packages, one for peering, one for syncing.
package hello

import (
	"fmt"
	"strings"

	"github.com/glothriel/wormhole/pkg/peers"
)

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

// NewPeerEnrichingAppSource creates a new AppSource that enriches the apps with the given peer
func NewPeerEnrichingAppSource(peer string, child AppSource) AppSource {
	return &peerEnrichingAppSource{
		peer:  peer,
		child: child,
	}
}

type addressEnrichingAppSource struct {
	hostname string
	child    AppSource
}

func (s *addressEnrichingAppSource) List() ([]peers.App, error) {
	theApps, err := s.child.List()
	if err != nil {
		return nil, err
	}
	newApps := make([]peers.App, len(theApps))
	for i := range theApps {
		segments := strings.Split(theApps[i].Address, ":")
		if len(segments) != 2 {
			return nil, fmt.Errorf("Invalid address: %s", theApps[i].Address)
		}

		segments[0] = s.hostname
		newApps[i] = peers.WithAddress(theApps[i], strings.Join(segments, ":"))
	}
	return newApps, nil
}

// NewAddressEnrichingAppSource creates a new AppSource that enriches the apps with the given hostname
func NewAddressEnrichingAppSource(hostname string, child AppSource) AppSource {
	return &addressEnrichingAppSource{
		hostname: hostname,
		child:    child,
	}
}
