package wg

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// generateKeyPair generates a new private/public key pair.
func generateKeyPair() ([32]byte, [32]byte, error) {
	// Those are base64 - encoded keys
	private, public, generateErr := GetOrGenerateKeyPair(NewNoStorage())
	if generateErr != nil {
		return [32]byte{}, [32]byte{}, generateErr
	}

	return ConvertFromString(private, public)
}

func ConvertFromString(private, public string) ([32]byte, [32]byte, error) {
	// decode base64 keys
	var privateKey, publicKey [32]byte

	rawPriv, decodePrivErr := base64.StdEncoding.DecodeString(private)
	if decodePrivErr != nil || len(rawPriv) != 32 {
		if decodePrivErr != nil {
			return [32]byte{}, [32]byte{}, decodePrivErr
		}
		return [32]byte{}, [32]byte{}, errors.New("private key is not 32 bytes long")
	}
	copy(privateKey[:], rawPriv)

	rawPub, decodePubErr := base64.StdEncoding.DecodeString(public)
	if decodePubErr != nil || len(rawPub) != 32 {
		if decodePubErr != nil {
			return [32]byte{}, [32]byte{}, decodePubErr
		}
		return [32]byte{}, [32]byte{}, errors.New("public key is not 32 bytes long")
	}
	copy(publicKey[:], rawPub)

	return privateKey, publicKey, nil
}

// PerformKeyExchange computes a shared secret using peer's public key and our private key.
func PerformKeyExchange(privateKey, peerPublicKey [32]byte) ([32]byte, error) {
	sharedSecret, err := curve25519.X25519(privateKey[:], peerPublicKey[:])
	if err != nil {
		return [32]byte{}, err
	}
	var sharedSecretArray [32]byte
	copy(sharedSecretArray[:], sharedSecret[:32])
	if sharedSecretArray == [32]byte{} {
		return [32]byte{}, errors.New("shared secret is all zeroes")
	}
	return sharedSecretArray, nil
}

// DeriveKeys derives encryption and authentication keys using HKDF.
func DeriveKeys(sharedSecret [32]byte) ([32]byte, [32]byte, error) {
	hkdf := hkdf.New(func() hash.Hash {
		theHash, hashErr := blake2s.New256(nil)
		if hashErr != nil {
			logrus.Errorf("Failed to create hash: %v", hashErr)
			return nil
		}
		return theHash
	}, sharedSecret[:], nil, nil)

	var encryptionKey, authenticationKey [32]byte
	_, err := io.ReadFull(hkdf, encryptionKey[:])
	if err != nil {
		return [32]byte{}, [32]byte{}, err
	}
	_, err = io.ReadFull(hkdf, authenticationKey[:])
	if err != nil {
		return [32]byte{}, [32]byte{}, err
	}
	return encryptionKey, authenticationKey, nil
}

// encrypt encrypts the payload using the provided encryption key and authentication key.
func encrypt(payload []byte, encryptionKey, authenticationKey [32]byte) ([]byte, error) {
	block, err := aes.NewCipher(encryptionKey[:])
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, payload, authenticationKey[:])
	return ciphertext, nil
}

// decrypt decrypts the payload using the provided encryption key and authentication key.
func decrypt(ciphertext []byte, encryptionKey, authenticationKey [32]byte) ([]byte, error) {
	block, err := aes.NewCipher(encryptionKey[:])
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	payload, err := aesGCM.Open(nil, nonce, ciphertext, authenticationKey[:])
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func Encrypt(payload []byte, private, public string) ([]byte, error) {
	privateKey, publicKey, err := ConvertFromString(private, public)
	if err != nil {
		return nil, err
	}
	sharedSecret, err := PerformKeyExchange(privateKey, publicKey)
	if err != nil {
		return nil, err
	}
	encryptionKey, authenticationKey, err := DeriveKeys(sharedSecret)
	if err != nil {
		return nil, err
	}
	return encrypt(payload, encryptionKey, authenticationKey)
}

func Decrypt(ciphertext []byte, private, public string) ([]byte, error) {
	privateKey, publicKey, err := ConvertFromString(private, public)
	if err != nil {
		return nil, err
	}
	sharedSecret, err := PerformKeyExchange(privateKey, publicKey)
	if err != nil {
		return nil, err
	}
	encryptionKey, authenticationKey, err := DeriveKeys(sharedSecret)
	if err != nil {
		return nil, err
	}
	return decrypt(ciphertext, encryptionKey, authenticationKey)
}
