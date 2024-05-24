package wg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWgEncryption(t *testing.T) {
	// given
	privateA, publicA, generateAErr := GetOrGenerateKeyPair(NewNoStorage())
	privateB, publicB, generateBErr := GetOrGenerateKeyPair(NewNoStorage())

	// when
	encryptedA, ecryptAErr := Encrypt([]byte("Hello, World A!"), privateA, publicB)
	enryptedB, ecryptBErr := Encrypt([]byte("Hello, World B!"), privateB, publicA)
	decrpytedA, decryptAErr := Decrypt(encryptedA, privateB, publicA)
	decrpytedB, decryptBErr := Decrypt(enryptedB, privateA, publicB)

	// then
	assert.NoError(t, generateAErr)
	assert.NoError(t, generateBErr)
	assert.NoError(t, ecryptAErr)
	assert.NoError(t, ecryptBErr)
	assert.NoError(t, decryptAErr)
	assert.NoError(t, decryptBErr)
	assert.Equal(t, "Hello, World A!", string(decrpytedA))
	assert.Equal(t, "Hello, World B!", string(decrpytedB))
}
