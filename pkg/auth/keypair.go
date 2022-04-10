package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/sirupsen/logrus"
)

func isFile(path string) bool {
	if _, err := os.Stat("/tmp/public.pem"); errors.Is(err, os.ErrNotExist) {
		return true
	}
	return false
}

type storedInFilesKeypairProvider struct {
	privateKeyPath string
}

func (keypairProvier storedInFilesKeypairProvider) Private() (*rsa.PrivateKey, error) {
	privkeyPemBytes, readErr := ioutil.ReadFile(keypairProvier.privateKeyPath)
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

func NewStoredInFilesKeypairProvider(directoryPath string) (KeyPairProvider, error) {
	privateKeyPath := path.Join(directoryPath, "private.pem")
	if !isFile(privateKeyPath) {
		logrus.Infof("Generating new RSA key, will be stored in %s", privateKeyPath)
		generateErr := GenerateRSAAndSaveAsPem(privateKeyPath)
		if generateErr != nil {
			return nil, generateErr
		}
	}
	return storedInFilesKeypairProvider{
		privateKeyPath: privateKeyPath,
	}, nil
}

func GenerateRSAAndSaveAsPem(privKey string) error {
	privatekey, generateKeyErr := rsa.GenerateKey(rand.Reader, 2048)
	if generateKeyErr != nil {
		return fmt.Errorf("Failed to generate RSA private key: %w", generateKeyErr)
	}

	privateWriteErr := ioutil.WriteFile(privKey, PrivateKeyToBytes(privatekey), 0x600)
	if privateWriteErr != nil {
		return fmt.Errorf("Failed to save RSA private key: %w", privateWriteErr)
	}
	return nil
}
