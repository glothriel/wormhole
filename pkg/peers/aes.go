package peers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"

	"github.com/sirupsen/logrus"
)

func encrypt(key string, data []byte) ([]byte, error) {
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

func decrypt(key string, data []byte) ([]byte, error) {
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

func ensureHas32Bytes(key string) []byte {
	keyComplement := "1234567890qwertyuiopasdfghjklzxc"
	if len(key) == 0 {
		logrus.Warning("Supplied encryption key is empty, please fix that...")
		key = keyComplement
	}
	if len(key) < 32 {
		key = key + keyComplement[:32-len(key)]
	}
	if len(key) > 32 {
		key = key[:32]
	}
	return []byte(key)
}
