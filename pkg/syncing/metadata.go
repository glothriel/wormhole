package syncing

import (
	"errors"
	"sync"
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
	data sync.Map
}

func (s *inMemoryMetadataStorage) List() ([]MetadataListItem, error) {
	var items []MetadataListItem
	s.data.Range(func(key, value any) bool {
		items = append(items, MetadataListItem{
			Peer:     key.(string),
			Metadata: value.(Metadata),
		})
		return true
	})
	return items, nil
}

func (s *inMemoryMetadataStorage) Set(peer string, metadata Metadata) error {
	s.data.Store(peer, metadata)
	return nil
}

func (s *inMemoryMetadataStorage) Get(peer string) (Metadata, error) {
	val, ok := s.data.Load(peer)
	if !ok {
		return nil, ErrPeerNotFound
	}
	return val.(Metadata), nil
}

// NewInMemoryMetadataStorage creates a new in-memory metadata storage
func NewInMemoryMetadataStorage() MetadataStorage {
	return &inMemoryMetadataStorage{}
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
