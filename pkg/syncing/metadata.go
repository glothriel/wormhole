package syncing

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

// ErrPeerNotFound is returned when a peer is not found in the metadata storage
var ErrPeerNotFound = errors.New("not found")

// Metadata allows storing arbitrary metadata in sync messages
type Metadata map[string]any

// MetadataListItem is used to present metadata in a list
type MetadataListItem struct {
	Peer     string
	Metadata Metadata
}

// MetadataStorage is an interface for storing and retrieving metadata
type MetadataStorage interface {
	List() ([]MetadataListItem, error)
	Set(peer string, metadata Metadata) error
	Get(peer string) (Metadata, error)
}

// MetadataFactory is an interface for creating metadata, can be used for
// example for loading mounted secrets in kubernetes and reloading metadata
// without restarting the application
type MetadataFactory interface {
	Get() (Metadata, error)
}

type inMemoryMetadataStorage struct {
	maxCapacity int
	mutex       sync.Mutex
	data        map[string]Metadata
}

func (s *inMemoryMetadataStorage) List() ([]MetadataListItem, error) {
	var items []MetadataListItem
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for peer, metadata := range s.data {
		items = append(items, MetadataListItem{
			Peer:     peer,
			Metadata: metadata,
		})
	}
	return items, nil
}

func (s *inMemoryMetadataStorage) Set(peer string, metadata Metadata) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.data[peer] = metadata
	if len(s.data) > s.maxCapacity {
		for peer := range s.data {
			logrus.Warnf("metadata storage is full, evicting %s", peer)
			delete(s.data, peer)
			break
		}
	}
	return nil
}

func (s *inMemoryMetadataStorage) Get(peer string) (Metadata, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	metadata, ok := s.data[peer]
	if !ok {
		return nil, ErrPeerNotFound
	}
	return metadata, nil
}

// NewInMemoryMetadataStorage creates a new in-memory metadata storage
func NewInMemoryMetadataStorage() MetadataStorage {
	return &inMemoryMetadataStorage{
		maxCapacity: 2048,
		data:        make(map[string]Metadata),
		mutex:       sync.Mutex{},
	}
}

type boltMetadataStorage struct {
	db *bolt.DB
}

func (s *boltMetadataStorage) List() ([]MetadataListItem, error) {
	var items []MetadataListItem
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("metadata"))
		return b.ForEach(func(k, v []byte) error {
			var metadata Metadata
			if unmarshalErr := json.Unmarshal(v, &metadata); unmarshalErr != nil {
				return unmarshalErr
			}
			items = append(items, MetadataListItem{
				Peer:     string(k),
				Metadata: metadata,
			})
			return nil
		})
	})
	return items, err
}

func (s *boltMetadataStorage) Set(peer string, metadata Metadata) error {
	metadataBytes, marshalErr := json.Marshal(metadata)
	if marshalErr != nil {
		return marshalErr
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("metadata"))
		return b.Put([]byte(peer), metadataBytes)
	})
}

func (s *boltMetadataStorage) Get(peer string) (Metadata, error) {
	var metadata Metadata
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("metadata"))
		metadataBytes := b.Get([]byte(peer))
		if metadataBytes == nil {
			return ErrPeerNotFound
		}
		return json.Unmarshal(metadataBytes, &metadata)
	})
	return metadata, err
}

// NewBoltMetadataStorage creates a new metadata storage that stores metadata in a BoltDB database
func NewBoltMetadataStorage(path string) (MetadataStorage, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	if updateErr := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("metadata"))
		return err
	}); updateErr != nil {
		return nil, updateErr
	}
	return &boltMetadataStorage{db}, nil
}

type cachingMetadataStorage struct {
	storage MetadataStorage
	cache   *inMemoryMetadataStorage
}

func (s *cachingMetadataStorage) List() ([]MetadataListItem, error) {
	return s.cache.List()
}

func (s *cachingMetadataStorage) Set(peer string, metadata Metadata) error {
	childErr := s.storage.Set(peer, metadata)
	if childErr != nil {
		return childErr
	}
	return s.cache.Set(peer, metadata)
}

func (s *cachingMetadataStorage) Get(peer string) (Metadata, error) {
	metadata, err := s.cache.Get(peer)
	if err == nil {
		return metadata, nil
	}
	if err != ErrPeerNotFound {
		return nil, err
	}
	metadata, err = s.storage.Get(peer)
	if err != nil {
		return nil, err
	}
	setErr := s.cache.Set(peer, metadata)
	if setErr != nil {
		logrus.Errorf("failed to cache metadata for peer %s: %v", peer, setErr)
	}
	return metadata, nil
}

// NewCachingMetadataStorage creates a new metadata storage that caches metadata in memory
func NewCachingMetadataStorage(storage MetadataStorage) MetadataStorage {
	return &cachingMetadataStorage{
		storage: storage,
		cache:   NewInMemoryMetadataStorage().(*inMemoryMetadataStorage),
	}
}

type staticMetadataFactory struct {
	metadata Metadata
}

func (f *staticMetadataFactory) Get() (Metadata, error) {
	return f.metadata, nil
}

// NewStaticMetadataFactory creates a new static metadata factory
func NewStaticMetadataFactory(metadata Metadata) MetadataFactory {
	return &staticMetadataFactory{
		metadata: metadata,
	}
}
