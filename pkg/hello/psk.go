package hello

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

type pskPairingServerTransport struct {
	psk   string
	child PairingServerTransport
}

func (t *pskPairingServerTransport) Requests() <-chan IncomingPairingRequest {
	theChan := make(chan IncomingPairingRequest)
	go func() {
		for childReq := range t.child.Requests() {
			decrypted, aesError := AesDecrypt([]byte(t.psk), childReq.Request)
			if aesError != nil {
				childReq.Err <- fmt.Errorf("failed to decrypt request: %v", aesError)
				continue
			}

			newRequest := IncomingPairingRequest{
				Request:  decrypted,
				Response: make(chan []byte),
				Err:      make(chan error),
			}

			go func() {
				select {
				case e := <-newRequest.Err:
					childReq.Err <- e
				case r := <-newRequest.Response:
					newResponse, aesError := AesEncrypt([]byte(t.psk), r)
					if aesError != nil {
						newRequest.Err <- fmt.Errorf("failed to encrypt response: %v", aesError)
						return
					}
					childReq.Response <- newResponse
				}
			}()
			theChan <- newRequest
		}
	}()
	return theChan
}

func NewPSKPairingServerTransport(psk string, child PairingServerTransport) PairingServerTransport {
	return &pskPairingServerTransport{
		child: child,
		psk:   psk,
	}
}

type pskPairingClientTransport struct {
	psk   string
	child PairingClientTransport
}

func (t *pskPairingClientTransport) Send(req []byte) ([]byte, error) {
	encrypted, aesError := AesEncrypt([]byte(t.psk), req)
	if aesError != nil {
		return nil, fmt.Errorf("failed to encrypt request: %v", aesError)
	}
	childResp, sendErr := t.child.Send(encrypted)
	if sendErr != nil {
		return nil, sendErr
	}
	decrypted, aesError := AesDecrypt([]byte(t.psk), childResp)
	if aesError != nil {
		return nil, fmt.Errorf("failed to decrypt response: %v", aesError)
	}
	return decrypted, nil

}

func NewPSKClientPairingTransport(psk string, child PairingClientTransport) PairingClientTransport {
	return &pskPairingClientTransport{
		child: child,
		psk:   psk,
	}
}

// staticKey is a slice of bytes to append to short keys.
var staticKey = []byte{0x15, 0x77, 0x7f, 0xc2, 0x94, 0xf9, 0xa7, 0xef, 0x2b, 0x57, 0x55, 0x53, 0x53, 0x7c, 0x10, 0x85}

// AesEncrypt encrypts plaintext using the provided pre-shared key (psk).
func AesEncrypt(psk []byte, plaintext []byte) ([]byte, error) {
	// Check key length and adjust if necessary
	key, err := adjustKeyLength(psk)
	if err != nil {
		return nil, err
	}

	// Create new AES cipher using the adjusted key
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create a new GCM cipher mode
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate a nonce for encryption
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt the data
	ciphertext := aesGCM.Seal(nil, nonce, plaintext, nil)
	// Append nonce to the ciphertext
	ciphertext = append(nonce, ciphertext...)

	return ciphertext, nil
}

// AesDecrypt decrypts ciphertext using the provided pre-shared key (psk).
func AesDecrypt(psk []byte, ciphertext []byte) ([]byte, error) {
	// Check key length and adjust if necessary
	key, err := adjustKeyLength(psk)
	if err != nil {
		return nil, err
	}

	// Create new AES cipher using the adjusted key
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create a new GCM cipher mode
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Split nonce and actual ciphertext
	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, encryptedMsg := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt the data
	plaintext, err := aesGCM.Open(nil, nonce, encryptedMsg, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// adjustKeyLength ensures the key is exactly 16 bytes long.
func adjustKeyLength(key []byte) ([]byte, error) {
	if len(key) < 6 {
		return nil, errors.New("key too short; must be at least 6 characters")
	}

	if len(key) > 16 {
		key = key[:16]
	} else if len(key) < 16 {
		key = append(key, staticKey[:16-len(key)]...)
	}

	return key, nil
}
