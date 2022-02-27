package peers

import (
	"testing"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func generateLocalConnectionAndRemoteTransport() (*DefaultPeer, *MockTransport) {
	remoteTransport, ochestratorTransport := CreateMockTransportPair()
	remoteTransport.Send(messages.NewIntroduction("test-remote-machine"))
	connection, connnectionErr := NewDefaultPeer("test-local-machine", ochestratorTransport)
	if connnectionErr != nil {
		logrus.Fatal(connnectionErr)
	}
	return connection, remoteTransport
}

func TestNewPeerConnectionIntroductionWorksCorrectly(t *testing.T) {
	connection, remoteTransport := generateLocalConnectionAndRemoteTransport()

	assert.Equal(t, "test-remote-machine", connection.Name())
	assert.Equal(t, "test-local-machine", (<-remoteTransport.inbox).BodyString)
}

func TestNewPeerConnectionErrorIsThrownWhenMessageOtherThanIntroductionIsReceived(t *testing.T) {
	remoteTransport, ochestratorTransport := CreateMockTransportPair()
	remoteTransport.Send(messages.NewPing())
	_, connnectionErr := NewDefaultPeer("test-local-machine", ochestratorTransport)

	assert.NotNil(t, connnectionErr)
}

func TestPeerConnectionProperlyPassesFramesToRemote(t *testing.T) {
	// given
	connection, remoteTransport := generateLocalConnectionAndRemoteTransport()
	theMessages := []messages.Message{
		messages.NewFrame("session-1", []byte("foo")),
		messages.NewFrame("session-1", []byte("bar")),
		messages.NewFrame("session-2", []byte("baz")),
	}

	// when
	for i := range theMessages {
		assert.Nil(t, connection.Send(theMessages[i]))
	}
	messagesComingToRemote, remoteReceiveErr := remoteTransport.Receive()

	// then
	assert.Nil(t, remoteReceiveErr)
	assert.True(t, messages.IsIntroduction(<-messagesComingToRemote))
	for i := range theMessages {
		assert.Equal(t, theMessages[i], <-messagesComingToRemote)
	}
}

func TestPeerConnectionProperlyPassesFramesFromRemote(t *testing.T) {
	// given
	connection, remoteTransport := generateLocalConnectionAndRemoteTransport()
	theMessages := []messages.Message{
		messages.NewFrame("session-1", []byte("foo")),
		messages.NewFrame("session-1", []byte("bar")),
		messages.NewFrame("session-2", []byte("baz")),
	}

	// when
	for i := range theMessages {
		assert.Nil(t, remoteTransport.Send(theMessages[i]))
	}
	messagesCommingFromRemoteToConnection := connection.Frames()

	// then
	for i := range theMessages {
		assert.Equal(t, theMessages[i], <-messagesCommingFromRemoteToConnection)
	}
}
