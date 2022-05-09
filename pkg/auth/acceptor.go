package auth

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
)

// DummyAcceptor implements Acceptor by blindly trusting all keys
type DummyAcceptor struct {
}

// IsTrusted implements Acceptor
func (a DummyAcceptor) IsTrusted(*rsa.PublicKey) (bool, error) {
	return true, nil
}

type inMemoryCachingAcceptor struct {
	entries sync.Map
	child   Acceptor
}

func (storage *inMemoryCachingAcceptor) IsTrusted(cert *rsa.PublicKey) (bool, error) {
	val, ok := storage.entries.Load(Fingerprint(cert))
	if ok {
		return val.(bool), nil
	}
	childResult, childErr := storage.child.IsTrusted(cert)
	if childErr != nil || !childResult {
		return false, childErr
	}
	storage.entries.Store(Fingerprint(cert), childResult)
	return childResult, nil
}

// NewInMemoryCachingAcceptor returns acceptor, that caches trusted fingerprints in memory
func NewInMemoryCachingAcceptor(child Acceptor) Acceptor {
	return &inMemoryCachingAcceptor{
		child:   child,
		entries: sync.Map{},
	}
}

type inFileCachingAcceptor struct {
	lock  *sync.Mutex
	path  string
	child Acceptor
}

func (storage *inFileCachingAcceptor) IsTrusted(cert *rsa.PublicKey) (bool, error) {
	cacheMap, readCacheErr := storage.getCache()
	if readCacheErr != nil {
		return false, readCacheErr
	}
	cachedIsTrustedResult, ok := cacheMap[Fingerprint(cert)]
	if ok {
		return cachedIsTrustedResult, nil
	}
	childResult, childErr := storage.child.IsTrusted(cert)
	if childErr != nil || !childResult {
		return false, childErr
	}

	return childResult, storage.locked(func() error {
		resultMap, readFileErr := storage.getCache()
		if readFileErr != nil {
			return readFileErr
		}
		resultMap[Fingerprint(cert)] = childResult

		return storage.setCache(resultMap)
	})
}

func (storage *inFileCachingAcceptor) locked(fn func() error) error {
	storage.lock.Lock()
	defer storage.lock.Unlock()
	return fn()
}

func (storage *inFileCachingAcceptor) getCache() (map[string]bool, error) {
	cacheMap := map[string]bool{}
	readBytes, readErr := ioutil.ReadFile(storage.path)
	if readErr != nil {
		if !os.IsNotExist(readErr) {
			return cacheMap, fmt.Errorf("Failed to read acceptor cache file: %w", readErr)
		}
		// If file doesn't exist, just use empty cache map
	} else {
		if unmarshalErr := json.Unmarshal(readBytes, &cacheMap); unmarshalErr != nil {
			return cacheMap, fmt.Errorf("Failed to parse acceptor cache file as JSON: %w", unmarshalErr)
		}
	}

	return cacheMap, nil
}

func (storage *inFileCachingAcceptor) setCache(cacheMap map[string]bool) error {
	theData, marshalErr := json.Marshal(cacheMap)
	if marshalErr != nil {
		return fmt.Errorf("Failed to encode acceptor cache to JSON: %w", marshalErr)
	}
	if writeErr := ioutil.WriteFile(storage.path, theData, 0600); writeErr != nil {
		return fmt.Errorf("Failed to write acceptor cache to file: %w", writeErr)
	}
	return nil
}

// NewInFileCachingAcceptor returns acceptor, that caches trusted fingerprints in file
func NewInFileCachingAcceptor(filePath string, child Acceptor) Acceptor {
	return &inFileCachingAcceptor{
		child: child,
		path:  filePath,
		lock:  &sync.Mutex{},
	}
}
