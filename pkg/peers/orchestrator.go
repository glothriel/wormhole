package peers

import (
	"fmt"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/sirupsen/logrus"
)

// PeerConnection implements Peer by plucing out Transport layer into another interface
type PeerConnection struct {
	remoteName string

	transport  Transport
	framesChan chan messages.Message
	appsChan   chan AppEvent
}

// Name implements Peer
func (o *PeerConnection) Name() string {
	return o.remoteName
}

// Receive implements Peer
func (o *PeerConnection) Receive() (chan messages.Message, error) {
	return o.Frames(), nil
}

// Frames returns messages that are used to interchange app data
func (o *PeerConnection) Frames() chan messages.Message {
	return o.framesChan
}

// AppEvents immplements Peer
func (o *PeerConnection) AppEvents() chan AppEvent {
	return o.appsChan
}

// Send immplements Peer
func (o *PeerConnection) Send(msg messages.Message) error {
	return o.transport.Send(msg)
}

// Close immplements Peer
func (o *PeerConnection) Close() error {
	return o.transport.Close()
}

// WhenClosed immplements Peer
func (o *PeerConnection) WhenClosed(func()) {
}

func (o *PeerConnection) startRouting(failedChan chan error, localName string) {
	messagesChan, receiveErr := o.transport.Receive()
	if receiveErr != nil {
		failedChan <- receiveErr
		return
	}
	o.transport.Send(messages.NewIntroduction(localName))

	logrus.Debug("A new peer detected, waiting for introduction")
	introductionMessage := <-messagesChan

	if !messages.IsIntroduction(introductionMessage) {
		o.transport.Close()
		logrus.Error(introductionMessage)
		failedChan <- fmt.Errorf(
			"New peer connected, but no introduction message received, closing remote connection: %v", introductionMessage,
		)
		return
	}
	failedChan <- nil
	o.remoteName = introductionMessage.BodyString
	logrus.Infof("Peer %s connected", o.remoteName)
	for message := range messagesChan {
		if messages.IsFrame(message) {
			o.framesChan <- message
		} else if messages.IsAppAdded(message) || messages.IsAppWithdrawn(message) {
			o.appsChan <- AppEvent{Type: message.Type, App: App{Name: message.BodyString}}
		} else if messages.IsDisconnect(message) {
			break
		}
	}
	close(o.framesChan)
	close(o.appsChan)
}

// NewPeerConnection creates PeerConnection instances
func NewPeerConnection(introduceAsName string, transport Transport) (*PeerConnection, error) {
	theConnection := &PeerConnection{
		transport:  transport,
		framesChan: make(chan messages.Message),
		appsChan:   make(chan AppEvent),
	}
	orchestrationFailed := make(chan error)
	go theConnection.startRouting(orchestrationFailed, introduceAsName)
	if orchestrationFailedErr := <-orchestrationFailed; orchestrationFailedErr != nil {
		return nil, orchestrationFailedErr
	}
	return theConnection, nil
}

// PeerConnectionFactory implements PeerFactory
type PeerConnectionFactory struct {
	ownName          string
	transportFactory TransportFactory
}

// Peers implements PeerFactory
func (peerConnectionFactory *PeerConnectionFactory) Peers() (chan Peer, error) {
	connections := make(chan Peer)
	go func() {
		for {
			newTransport, newTransportErr := peerConnectionFactory.transportFactory.Create()
			if newTransportErr != nil {
				close(connections)
				return
			}
			newConnection, newConnectionErr := NewPeerConnection(peerConnectionFactory.ownName, newTransport)
			if newConnectionErr != nil {
				close(connections)
				return
			}
			connections <- newConnection
		}
	}()
	return connections, nil
}

// NewPeerConnectionFactory creates PeerConnectionFactory instances
func NewPeerConnectionFactory(ownName string, transportFactory TransportFactory) PeerFactory {
	return &PeerConnectionFactory{transportFactory: transportFactory, ownName: ownName}
}
