package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"
)

func isFile(path string) bool {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

type storedInFilesKeypairProvider struct {
	privateKeyPath string
}

func (keypairProvier storedInFilesKeypairProvider) Private() (*rsa.PrivateKey, error) {
	privkeyPemBytes, readErr := os.ReadFile(keypairProvier.privateKeyPath)
	if readErr != nil {
		return nil, fmt.Errorf("Failed to read private RSA key: %w", readErr)
	}
	return BytesToPrivateKey(privkeyPemBytes)
}

func (keypairProvier storedInFilesKeypairProvider) Public() (*rsa.PublicKey, error) {
	privateKey, privateKeyErr := keypairProvier.Private()
	if privateKeyErr != nil {
		return nil, fmt.Errorf("Failed to read public RSA key: %w", privateKeyErr)
	}
	return &privateKey.PublicKey, nil
}

// NewStoredInFilesKeypairProvider uses private key from given directory or creates fresh one
// if none exists, then uses it as KeypairProvider.
func NewStoredInFilesKeypairProvider(directoryPath string) (KeypairProvider, error) {
	privateKeyPath := path.Join(directoryPath, "private.pem")
	if !isFile(privateKeyPath) {
		logrus.Infof("Generating new RSA key, will be stored in %s", privateKeyPath)
		generateErr := generateRSAAndSaveAsPem(privateKeyPath)
		if generateErr != nil {
			return nil, generateErr
		}
	}
	return storedInFilesKeypairProvider{
		privateKeyPath: privateKeyPath,
	}, nil
}

func generateRSAAndSaveAsPem(privKey string) error {
	privatekey, generateKeyErr := rsa.GenerateKey(rand.Reader, 2048)
	if generateKeyErr != nil {
		return fmt.Errorf("Failed to generate RSA private key: %w", generateKeyErr)
	}

	privateWriteErr := os.WriteFile(privKey, PrivateKeyToBytes(privatekey), 0600)
	if privateWriteErr != nil {
		return fmt.Errorf("Failed to save RSA private key: %w", privateWriteErr)
	}
	return nil
}
