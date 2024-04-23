package hello

import (
	"fmt"
	"sync"

	"github.com/glothriel/wormhole/pkg/peers"
)

type PeerStorage interface {
	Store(PeerInfo) error
	GetByName(string) (PeerInfo, error)
	GetByIP(string) (PeerInfo, error)
	List() ([]PeerInfo, error)
}

type inMemoryPeerStorage struct {
	peers sync.Map
}

func (s *inMemoryPeerStorage) Store(peer PeerInfo) error {
	s.peers.Store(peer.Name, peer)
	return nil
}

func (s *inMemoryPeerStorage) GetByName(name string) (PeerInfo, error) {
	if peer, ok := s.peers.Load(name); ok {
		return peer.(PeerInfo), nil
	}
	return PeerInfo{}, fmt.Errorf("peer with name %s not found", name)
}

func (s *inMemoryPeerStorage) GetByIP(ip string) (PeerInfo, error) {
	var found PeerInfo
	s.peers.Range(func(_, value interface{}) bool {
		peer := value.(PeerInfo)
		if peer.IP == ip {
			found = peer
			return false
		}
		return true
	})
	if found.Name == "" {
		return PeerInfo{}, fmt.Errorf("peer with IP %s not found", ip)
	}
	return found, nil
}

func (s *inMemoryPeerStorage) List() ([]PeerInfo, error) {
	var peers []PeerInfo
	s.peers.Range(func(_, value interface{}) bool {
		peers = append(peers, value.(PeerInfo))
		return true
	})
	return peers, nil
}

func NewInMemoryPeerStorage() PeerStorage {
	return &inMemoryPeerStorage{}
}

type AppSource interface {
	List() ([]peers.App, error)
}

type inMemoryAppStorage struct {
	apps sync.Map
}

func (s *inMemoryAppStorage) Store(app peers.App) error {
	s.apps.Store(app.Peer+app.Name, app)
	return nil
}

func (s *inMemoryAppStorage) Remove(peer string, name string) error {
	s.apps.Delete(peer + name)
	return nil
}

func (s *inMemoryAppStorage) Get(peer string, name string) (peers.App, error) {
	if app, ok := s.apps.Load(peer + name); ok {
		return app.(peers.App), nil
	}
	return peers.App{}, fmt.Errorf("app with name %s not found", name)
}

func (s *inMemoryAppStorage) List() ([]peers.App, error) {
	var apps []peers.App
	s.apps.Range(func(_, value interface{}) bool {
		apps = append(apps, value.(peers.App))
		return true
	})
	return apps, nil
}

func NewInMemoryAppStorage() AppSource {
	return &inMemoryAppStorage{}
}
