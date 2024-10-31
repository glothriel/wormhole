package pairing

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

// ErrPeerDoesNotExist is returned when a peer does not exist yet
var ErrPeerDoesNotExist = errors.New("peer does not exist")

// PeerStorage is an interface for storing and retrieving peers
type PeerStorage interface {
	Store(PeerInfo) error
	GetByName(string) (PeerInfo, error)
	List() ([]PeerInfo, error)
	DeleteByName(string) error
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
	return PeerInfo{}, ErrPeerDoesNotExist
}

func (s *inMemoryPeerStorage) List() ([]PeerInfo, error) {
	var peers []PeerInfo
	s.peers.Range(func(_, value any) bool {
		peers = append(peers, value.(PeerInfo))
		return true
	})
	return peers, nil
}

func (s *inMemoryPeerStorage) DeleteByName(name string) error {
	s.peers.Delete(name)
	return nil
}

// NewInMemoryPeerStorage creates a new in-memory PeerStorage instance
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

func (s *boltPeerStorage) DeleteByName(name string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("peers"))
		return b.Delete([]byte(name))
	})
}

// NewBoltPeerStorage creates a new BoltDB (persistent, on-disk storage) PeerStorage instance
func NewBoltPeerStorage(path string) PeerStorage {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		logrus.Panicf("failed to open bolt db: %v", err)
	}
	if updateErr := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("peers"))
		return err
	}); updateErr != nil {
		logrus.Panicf("failed to create BoltDB bucket: %v", updateErr)
	}
	return &boltPeerStorage{db: db}
}
