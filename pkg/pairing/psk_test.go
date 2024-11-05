package pairing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptDecrypt(t *testing.T) {
	// given
	key := "1337huehuehue"
	plaintext := "Hello, World!"

	// when
	ciphertext, encryptErr := AesEncrypt([]byte(key), []byte(plaintext))
	decryptedPlaintext, decryptErr := AesDecrypt([]byte(key), ciphertext)

	// then
	assert.NoError(t, encryptErr)
	assert.NoError(t, decryptErr)
	assert.Equal(t, plaintext, string(decryptedPlaintext))
}
