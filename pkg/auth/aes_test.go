package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAesHelpers(t *testing.T) {
	// given
	originalPlaintext := "Hej, hej, hej! Sokoły! Omijajcie góry, lasy, doły!"
	theKey := generateAESKey()

	// when
	ciphertext, encryptErr := encrypt(theKey, []byte(originalPlaintext))
	assert.Nil(t, encryptErr)
	plaintext, decryptErr := decrypt(theKey, ciphertext)
	assert.Nil(t, decryptErr)

	// then
	assert.Equal(t, originalPlaintext, string(plaintext))
}
