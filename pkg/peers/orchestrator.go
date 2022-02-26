package peers

import (
	"fmt"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/sirupsen/logrus"
)

// DefaultPeer implements Peer by plucing out Transport layer into another interface
type DefaultPeer struct {
	remoteName string

	transport  Transport
	framesChan chan messages.Message
	appsChan   chan AppEvent
}

// Name implements Peer
func (o *DefaultPeer) Name() string {
	return o.remoteName
}

// Frames returns messages that are used to interchange app data
func (o *DefaultPeer) Frames() chan messages.Message {
	return o.framesChan
}

// AppEvents immplements Peer
func (o *DefaultPeer) AppEvents() chan AppEvent {
	return o.appsChan
}

// Send immplements Peer
func (o *DefaultPeer) Send(msg messages.Message) error {
	return o.transport.Send(msg)
}

// Close immplements Peer
func (o *DefaultPeer) Close() error {
	return o.transport.Close()
}

func (o *DefaultPeer) startRouting(failedChan chan error, localName string) {
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

// NewDefaultPeer creates PeerConnection instances
func NewDefaultPeer(introduceAsName string, transport Transport) (*DefaultPeer, error) {
	theConnection := &DefaultPeer{
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

// DefaultPeerFactory implements PeerFactory
type DefaultPeerFactory struct {
	ownName          string
	transportFactory TransportFactory
}

// Peers implements PeerFactory
func (defaultPeerFactory *DefaultPeerFactory) Peers() (chan Peer, error) {
	connections := make(chan Peer)
	go func() {
		for {
			newTransport, newTransportErr := defaultPeerFactory.transportFactory.Create()
			if newTransportErr != nil {
				close(connections)
				return
			}
			newConnection, newConnectionErr := NewDefaultPeer(defaultPeerFactory.ownName, newTransport)
			if newConnectionErr != nil {
				close(connections)
				return
			}
			connections <- newConnection
		}
	}()
	return connections, nil
}

// NewDefaultPeerFactory creates PeerConnectionFactory instances
func NewDefaultPeerFactory(ownName string, transportFactory TransportFactory) PeerFactory {
	return &DefaultPeerFactory{transportFactory: transportFactory, ownName: ownName}
}
