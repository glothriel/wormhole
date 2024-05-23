package hello

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

var ErrPeerDoesNotExist = errors.New("peer does not exist")

type PeerStorage interface {
	Store(PeerInfo) error
	GetByName(string) (PeerInfo, error)
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

type boltPeerStorage struct {
	db *bolt.DB
}

func (s *boltPeerStorage) Store(peer PeerInfo) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("peers"))
		encoded, encodeErr := json.Marshal(peer)
		if encodeErr != nil {
			return encodeErr
		}
		return b.Put([]byte(peer.Name), encoded)
	})
}

func (s *boltPeerStorage) GetByName(name string) (PeerInfo, error) {
	var peer PeerInfo
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("peers"))
		payload := b.Get([]byte(name))
		if payload == nil {
			return ErrPeerDoesNotExist
		}
		var p PeerInfo
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
		peer = p
		return nil
	})
	return peer, err
}

func (s *boltPeerStorage) List() ([]PeerInfo, error) {
	var peers []PeerInfo
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("peers"))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var p PeerInfo
			if err := json.Unmarshal(v, &p); err != nil {
				return err
			}
			peers = append(peers, p)
		}
		return nil
	})
	return peers, err
}

func NewBoltPeerStorage(path string) PeerStorage {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		logrus.Panicf("failed to open bolt db: %v", err)
	}
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("peers"))
		return err
	})
	return &boltPeerStorage{db: db}
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
