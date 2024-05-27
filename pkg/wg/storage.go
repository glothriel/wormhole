package wg

import (
	"errors"

	"github.com/sirupsen/logrus"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	bolt "go.etcd.io/bbolt"
)

// KeyStorage is responsible for storing and loading WireGuard key pair
type KeyStorage interface {
	Store(private, public string) error
	Load() (private, public string, err error)
}

type boltDbKeyStorage struct {
	db *bolt.DB
}

func (s *boltDbKeyStorage) Store(private, public string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("keys"))
		if err := b.Put([]byte("private"), []byte(private)); err != nil {
			return err
		}
		return b.Put([]byte("public"), []byte(public))
	})
}

func (s *boltDbKeyStorage) Load() (private, public string, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("keys"))
		private = string(b.Get([]byte("private")))
		public = string(b.Get([]byte("public")))
		return nil
	})
	if private == "" || public == "" {
		return "", "", errors.New("no keys stored")
	}
	return private, public, err
}

// NewBoltKeyStorage creates a new KeyStorage that stores keys in a BoltDB database
func NewBoltKeyStorage(path string) KeyStorage {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		logrus.Panicf("failed to open bolt db: %v", err)
	}
	if updateErr := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("keys"))
		return err
	}); updateErr != nil {
		logrus.Panicf("failed to create bucket: %v", updateErr)
	}
	return &boltDbKeyStorage{db}
}

type inMemoryKeyStorage struct {
	private, public string
}

func (s *inMemoryKeyStorage) Store(private, public string) error {
	s.private, s.public = private, public
	return nil
}

func (s *inMemoryKeyStorage) Load() (private, public string, err error) {
	if s.private == "" || s.public == "" {
		return "", "", errors.New("no keys stored")
	}
	return s.private, s.public, nil
}

type noStorage struct{}

func (s *noStorage) Store(_, _ string) error {
	return nil
}

func (s *noStorage) Load() (_, _ string, err error) {
	return "", "", errors.New("no storage")
}

// NewNoStorage creates a new KeyStorage that does not store keys
func NewNoStorage() KeyStorage {
	return &noStorage{}
}

// NewInMemoryKeyStorage creates a new KeyStorage that stores keys in memory
func NewInMemoryKeyStorage() KeyStorage {
	return &inMemoryKeyStorage{}
}

// GetOrGenerateKeyPair returns the stored key pair or generates a new one
func GetOrGenerateKeyPair(storage KeyStorage) (string, string, error) {
	private, public, err := storage.Load()
	if err == nil {
		return private, public, nil
	}
	pkey, keyErr := wgtypes.GeneratePrivateKey()
	if keyErr != nil {
		return "", "", keyErr
	}

	private, public = pkey.String(), pkey.PublicKey().String()

	if err := storage.Store(private, public); err != nil {
		return "", "", err
	}

	return private, public, nil
}
