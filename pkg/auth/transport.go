package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
)

// KeypairProvider allows retrieving key pairs for transport messages encryption
type KeypairProvider interface {
	Public() (*rsa.PublicKey, error)
	Private() (*rsa.PrivateKey, error)
}

// rsaAuthorizedTransport is a decorator over Transport interface, that encrypts all the messages in transit
type rsaAuthorizedTransport struct {
	child            peers.Transport
	childReceiveChan chan messages.Message
	password         []byte
}

// CiphertextTag prefixes all messages that have body encrypted
const CiphertextTag = "AesCiphertext::"

// Send implements Transport
func (transport *rsaAuthorizedTransport) Send(message messages.Message) error {
	cipherText, encryptErr := encrypt(
		transport.password, []byte(message.BodyString),
	)
	if encryptErr != nil {
		return encryptErr
	}
	return transport.child.Send(
		messages.WithBody(
			message,
			strings.Join([]string{CiphertextTag, base64.RawStdEncoding.EncodeToString(cipherText)}, ""),
		),
	)
}

// Receive implements Transport
func (transport *rsaAuthorizedTransport) Receive() (chan messages.Message, error) {
	localChan := make(chan messages.Message)
	var childChan chan messages.Message
	if transport.childReceiveChan != nil {
		childChan = transport.childReceiveChan
	} else {
		var childReceiveErr error
		childChan, childReceiveErr = transport.child.Receive()
		if childReceiveErr != nil {
			return nil, childReceiveErr
		}
	}
	go func() {
		for remoteMessage := range childChan {
			encryptedBase64, base64Err := base64.RawStdEncoding.DecodeString(remoteMessage.BodyString[len(CiphertextTag):])
			if base64Err != nil {
				logrus.Errorf("Could not decode base64: %v", base64Err)
			}

			plainText, decryptErr := decrypt(
				transport.password, encryptedBase64,
			)
			if decryptErr != nil {
				logrus.Errorf("Could not decrypt BodyString of incoming message: %v", decryptErr)
				continue
			}
			localChan <- messages.WithBody(remoteMessage, string(plainText))
		}
		close(localChan)
	}()
	return localChan, nil
}

// Close implements Transport
func (transport *rsaAuthorizedTransport) Close() error {
	return transport.child.Close()
}

// NewRSAAuthorizedTransport creates AesTranport instances
func NewRSAAuthorizedTransport(child peers.Transport, keyProvider KeypairProvider) (peers.Transport, error) {
	publicKey, publicKeyErr := keyProvider.Public()
	if publicKeyErr != nil {
		return nil, fmt.Errorf("Failed to fetch public key: %w", publicKeyErr)
	}
	encodedPublicKey, encodeErr := PublicKeyToBytes(publicKey)
	if encodeErr != nil {
		return nil, fmt.Errorf("Failed encoding public key to PEM: %w", encodeErr)
	}
	logrus.Infof(
		"Sending public key to the server, please make sure, that the fingerprint matches: %s",
		Fingerprint(publicKey),
	)
	if sendErr := child.Send(
		messages.Message{
			Type:       "RSA-PING",
			BodyString: base64.StdEncoding.EncodeToString(encodedPublicKey),
		},
	); sendErr != nil {
		return nil, sendErr
	}
	msgs, receiveErr := child.Receive()
	if receiveErr != nil {
		return nil, fmt.Errorf("Failed to receive the messages from child peer: %w", receiveErr)
	}
	pongMessage := <-msgs
	if pongMessage.Type != "RSA-PONG" {
		return nil, fmt.Errorf(
			"RSAAuthorizedTransport expects first message coming from server transport to be %s, got %s",
			"RSA-PONG",
			pongMessage.Type,
		)
	}
	encryptedPayload, decodeErr := base64.StdEncoding.DecodeString(pongMessage.BodyString)
	if decodeErr != nil {
		return nil, fmt.Errorf(
			"Failed to decode RSA-PONG message from base64: %s", decodeErr,
		)
	}

	privateKey, privateKeyErr := keyProvider.Private()
	if privateKeyErr != nil {
		return nil, fmt.Errorf("Failed to fetch private key: %w", privateKeyErr)
	}
	aesKey, aesKeyErr := DecryptWithPrivateKey(encryptedPayload, privateKey)
	if aesKeyErr != nil {
		return nil, fmt.Errorf(
			"Could not decrypt the AES key received from remote peer: %w", aesKeyErr,
		)
	}
	return &rsaAuthorizedTransport{
		child:            child,
		childReceiveChan: msgs,
		password:         aesKey,
	}, nil
}

// Acceptor lets rsaAuthorizedTransportFactory decide if the key is trusted or not
type Acceptor interface {
	IsTrusted(*rsa.PublicKey) (bool, error)
}

type rsaAuthorizedTransportFactory struct {
	child      peers.TransportFactory
	transports chan peers.Transport
	acceptor   Acceptor
}

func (factory *rsaAuthorizedTransportFactory) Transports() (chan peers.Transport, error) {
	myTransports := make(chan peers.Transport)

	childTransports, transportErr := factory.child.Transports()
	if transportErr != nil {
		return nil, transportErr
	}

	go func() {
		for transport := range childTransports {
			go func(transport peers.Transport) {
				initializedTransport, initializeErr := factory.initializeTransport(transport)

				if initializeErr == nil {
					myTransports <- initializedTransport
				} else {
					logrus.Warnf(
						"Did not initialize transport: %v", initializeErr,
					)
				}
			}(transport)
		}
		close(myTransports)
	}()

	return myTransports, nil
}

func (factory *rsaAuthorizedTransportFactory) initializeTransport(transport peers.Transport) (peers.Transport, error) {
	msgs, receiveErr := transport.Receive()
	if receiveErr != nil {
		return nil, factory.logClose(receiveErr, transport)
	}

	pingMessage := <-msgs

	if pingMessage.Type != "RSA-PING" {
		return nil, factory.logClose(fmt.Errorf(
			"RSAAuthorizedTransport expects first message coming from client transport to be `%s`, got `%s`",
			"RSA-PING",
			pingMessage.Type,
		), transport)
	}
	decoded, decodeErr := base64.StdEncoding.DecodeString(pingMessage.BodyString)
	if decodeErr != nil {
		return nil, factory.logClose(fmt.Errorf(
			"Could not decode RSA-PING message from base64: %w", decodeErr,
		), transport)
	}
	publicKey, publicKeyErr := BytesToPublicKey(decoded)
	if publicKeyErr != nil {
		return nil, factory.logClose(fmt.Errorf(
			"Could not extract a valid public key from RSA-PING message: %w", publicKeyErr,
		), transport)
	}

	isPublicKeyTrusted, isTrustedErr := factory.acceptor.IsTrusted(publicKey)
	if isTrustedErr != nil {
		return nil, factory.logClose(isTrustedErr, transport)
	}
	if !isPublicKeyTrusted {
		return nil, factory.logClose(errors.New("Key fingerprint is not trusted"), transport)
	}
	aesKey, aesErr := generateAESKey()
	if aesErr != nil {
		return nil, factory.logClose(aesErr, transport)
	}
	encryptedMessage, encryptErr := EncryptWithPublicKey(aesKey, publicKey)
	if encryptErr != nil {
		return nil, factory.logClose(
			fmt.Errorf("Failed to encrypt AES key with remote public key: %w", encryptErr),
			transport,
		)
	}
	if sendErr := transport.Send(
		messages.Message{
			Type:       "RSA-PONG",
			BodyString: base64.StdEncoding.EncodeToString(encryptedMessage),
		},
	); sendErr != nil {
		return nil, factory.logClose(sendErr, transport)
	}
	return &rsaAuthorizedTransport{
		password:         aesKey,
		child:            transport,
		childReceiveChan: msgs,
	}, nil
}

func (factory *rsaAuthorizedTransportFactory) logClose(upstreamErr error, t peers.Transport) error {
	if closeErr := t.Close(); closeErr != nil {
		logrus.Warnf("Problem when closing transport: %v", closeErr)
	}
	return upstreamErr
}

// NewRSAAuthorizedTransportFactory is a decorator over TransportFactory, that allows encryption in transit with AES
func NewRSAAuthorizedTransportFactory(child peers.TransportFactory, acceptor Acceptor) peers.TransportFactory {
	transportsChan := make(chan peers.Transport)

	return &rsaAuthorizedTransportFactory{
		transports: transportsChan,
		child:      child,
		acceptor:   acceptor,
	}
}
