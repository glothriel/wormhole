// Package syncing provides a way to sync apps between peers
package syncing

import (
	"fmt"
	"strings"
	"sync"

	"github.com/glothriel/wormhole/pkg/apps"
)

// AppSource is an interface for listing apps
type AppSource interface {
	List() ([]apps.App, error)
}

type peerEnrichingAppSource struct {
	peer  string
	child AppSource
}

func (s *peerEnrichingAppSource) List() ([]apps.App, error) {
	theApps, err := s.child.List()
	if err != nil {
		return nil, err
	}
	newApps := make([]apps.App, len(theApps))
	for i := range theApps {
		newApps[i] = apps.WithPeer(theApps[i], s.peer)
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

func (s *addressEnrichingAppSource) List() ([]apps.App, error) {
	theApps, err := s.child.List()
	if err != nil {
		return nil, err
	}
	newApps := make([]apps.App, len(theApps))
	for i := range theApps {
		segments := strings.Split(theApps[i].Address, ":")
		if len(segments) != 2 {
			return nil, fmt.Errorf("invalid address: %s", theApps[i].Address)
		}

		segments[0] = s.hostname
		newApps[i] = apps.WithAddress(theApps[i], strings.Join(segments, ":"))
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

type inMemoryAppStorage struct {
	apps sync.Map
}

func (s *inMemoryAppStorage) Store(app apps.App) error {
	s.apps.Store(app.Peer+app.Name, app)
	return nil
}

func (s *inMemoryAppStorage) Remove(peer string, name string) error {
	s.apps.Delete(peer + name)
	return nil
}

func (s *inMemoryAppStorage) Get(peer string, name string) (apps.App, error) {
	if app, ok := s.apps.Load(peer + name); ok {
		return app.(apps.App), nil
	}
	return apps.App{}, fmt.Errorf("app with name %s not found", name)
}

func (s *inMemoryAppStorage) List() ([]apps.App, error) {
	var theApps []apps.App
	s.apps.Range(func(_, value any) bool {
		theApps = append(theApps, value.(apps.App))
		return true
	})
	return theApps, nil
}

// NewInMemoryAppStorage creates a new in-memory AppSource instance
func NewInMemoryAppStorage() AppSource {
	return &inMemoryAppStorage{}
}
