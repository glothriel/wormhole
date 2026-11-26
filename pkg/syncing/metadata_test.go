package syncing

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryMetadataStorageDelete(t *testing.T) {
	storage := NewInMemoryMetadataStorage()
	require.NoError(t, storage.Set("peer1", Metadata{"key": "value1"}))
	require.NoError(t, storage.Set("peer2", Metadata{"key": "value2"}))

	require.NoError(t, storage.Delete("peer1"))

	_, err := storage.Get("peer1")
	assert.ErrorIs(t, err, ErrPeerNotFound)
	metadata, err := storage.Get("peer2")
	require.NoError(t, err)
	assert.Equal(t, Metadata{"key": "value2"}, metadata)
}

func TestInMemoryMetadataStorageDeleteUnknownPeer(t *testing.T) {
	storage := NewInMemoryMetadataStorage()

	assert.NoError(t, storage.Delete("unknown"))
}

func TestBoltMetadataStorageDeleteIsPersisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.db")
	storage, err := NewBoltMetadataStorage(path)
	require.NoError(t, err)
	require.NoError(t, storage.Set("peer1", Metadata{"key": "value1"}))
	require.NoError(t, storage.Set("peer2", Metadata{"key": "value2"}))

	require.NoError(t, storage.Delete("peer1"))
	require.NoError(t, storage.(*boltMetadataStorage).db.Close())

	reopened, err := NewBoltMetadataStorage(path)
	require.NoError(t, err)
	defer reopened.(*boltMetadataStorage).db.Close()
	_, err = reopened.Get("peer1")
	assert.ErrorIs(t, err, ErrPeerNotFound)
	metadata, err := reopened.Get("peer2")
	require.NoError(t, err)
	assert.Equal(t, Metadata{"key": "value2"}, metadata)
}

func TestCachingMetadataStorageDeleteRemovesFromCacheAndChild(t *testing.T) {
	child, err := NewBoltMetadataStorage(filepath.Join(t.TempDir(), "metadata.db"))
	require.NoError(t, err)
	defer child.(*boltMetadataStorage).db.Close()
	require.NoError(t, child.Set("peer1", Metadata{"key": "value1"}))
	storage := NewCachingMetadataStorage(child)
	_, err = storage.Get("peer1")
	require.NoError(t, err)

	require.NoError(t, storage.Delete("peer1"))

	_, err = storage.Get("peer1")
	assert.ErrorIs(t, err, ErrPeerNotFound)
	_, err = child.Get("peer1")
	assert.ErrorIs(t, err, ErrPeerNotFound)
}
