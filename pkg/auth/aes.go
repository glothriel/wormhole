package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

func encrypt(key []byte, data []byte) ([]byte, error) {
	theCipher, newCipherErr := aes.NewCipher(ensureHas32Bytes(key))
	if newCipherErr != nil {
		return []byte{}, newCipherErr
	}
	gcm, gcmErr := cipher.NewGCM(theCipher)
	if gcmErr != nil {
		return []byte{}, gcmErr
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, readErr := io.ReadFull(rand.Reader, nonce); readErr != nil {
		return []byte{}, readErr
	}
	return gcm.Seal(nonce, nonce, data, nil), nil
}

func decrypt(key []byte, data []byte) ([]byte, error) {
	theCipher, newCipherErr := aes.NewCipher(ensureHas32Bytes(key))
	if newCipherErr != nil {
		return []byte{}, newCipherErr
	}
	gcm, gcmErr := cipher.NewGCM(theCipher)
	if gcmErr != nil {
		return []byte{}, gcmErr
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return []byte{}, gcmErr
	}
	nonce, data := data[:nonceSize], data[nonceSize:]
	plaintext, gcmOpenErr := gcm.Open(nil, nonce, data, nil)
	if gcmOpenErr != nil {
		return []byte{}, gcmOpenErr
	}
	return plaintext, nil
}

func generateAESKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, readErr := rand.Read(key); readErr != nil {
		return key, fmt.Errorf("Unable to generate AES key: %w", readErr)
	}
	return key, nil
}

func ensureHas32Bytes(key []byte) []byte {
	keyComplement := []byte("1234567890qwertyuiopasdfghjklzxc")
	if len(key) == 0 {
		logrus.Warning("Supplied encryption key is empty, please fix that...")
		key = keyComplement
	}
	if len(key) < 32 {
		key = append(key, keyComplement[:32-len(key)]...)
	}
	if len(key) > 32 {
		key = key[:32]
	}
	return key
}
