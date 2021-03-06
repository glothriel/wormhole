package auth

import (
	"testing"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type mockTransportFactory struct {
	createdTransport peers.Transport
}

func (mock *mockTransportFactory) Transports() (chan peers.Transport, error) {
	transports := make(chan peers.Transport)
	go func() {
		transports <- mock.createdTransport
	}()
	return transports, nil
}

func TestMessagesArePassedTransparentlyToThePeers(t *testing.T) {
	// given
	clientMock, serverMock := peers.CreateMockTransportPair()
	clientTransportReady := make(chan peers.Transport)
	go func(returnChan chan peers.Transport) {
		keyPairProvider, kppErr := NewStoredInFilesKeypairProvider("/tmp")
		if kppErr != nil {
			logrus.Fatal(kppErr)
		}
		clientTransport, transportErr := NewRSAAuthorizedTransport(clientMock, keyPairProvider)
		if transportErr != nil {
			logrus.Fatal(transportErr)
		}
		returnChan <- clientTransport
	}(clientTransportReady)
	serverFactory := NewRSAAuthorizedTransportFactory(&mockTransportFactory{createdTransport: serverMock}, DummyAcceptor{})
	serverTransportChan, _ := serverFactory.Transports()
	clientTransport := <-clientTransportReady

	serverTransport := <-serverTransportChan

	// when
	assert.Nil(t, serverTransport.Send(messages.NewFrame(
		"unknown",
		[]byte("Cześć!"),
	)))
	assert.Nil(t, clientTransport.Send(messages.NewFrame(
		"unknown",
		[]byte("No hej!"),
	)))
	clientReceivedMessages, _ := clientTransport.Receive()
	serverReceivedMessages, _ := serverTransport.Receive()

	// then
	assert.Equal(t, "Cześć!", (<-clientReceivedMessages).BodyString)
	assert.Equal(t, "No hej!", (<-serverReceivedMessages).BodyString)
}
