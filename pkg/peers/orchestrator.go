package peers

import (
	"fmt"
	"time"

	"github.com/glothriel/wormhole/pkg/grtn"
	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/sirupsen/logrus"
)

// DefaultPeer implements Peer by plucing out Transport layer into another interface
type DefaultPeer struct {
	remoteName string

	transport     Transport
	framesChan    chan messages.Message
	sessionsChan  chan messages.Message
	appsChan      chan AppEvent
	closePingChan chan bool
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

// SessionEvents immplements Peer
func (o *DefaultPeer) SessionEvents() chan messages.Message {
	return o.sessionsChan
}

// Send immplements Peer
func (o *DefaultPeer) Send(msg messages.Message) error {
	return o.transport.Send(msg)
}

// Close immplements Peer
func (o *DefaultPeer) Close() error {
	return o.transport.Close()
}

func (o *DefaultPeer) startRouting(failedChan chan error, localName string) func() {
	return func() {
		messagesChan, receiveErr := o.transport.Receive()
		if receiveErr != nil {
			failedChan <- receiveErr
			return
		}

		if sendErr := o.transport.Send(messages.NewIntroduction(localName)); sendErr != nil {
			failedChan <- sendErr
			return
		}

		logrus.Debug("A new peer detected, waiting for introduction")
		introductionMessage := <-messagesChan

		if !messages.IsIntroduction(introductionMessage) {
			if closeErr := o.transport.Close(); closeErr != nil {
				logrus.Warnf("Failed to close the transport: %s", closeErr)
			}
			logrus.Error(introductionMessage)
			failedChan <- fmt.Errorf(
				"New peer connected, but no introduction message received, closing remote connection: %v", introductionMessage,
			)
			return
		}
		failedChan <- nil
		o.remoteName = introductionMessage.BodyString
		grtn.Go(o.startPinging())
		for message := range messagesChan {
			if messages.IsFrame(message) || messages.IsSessionClosed(message) {
				o.framesChan <- message
			} else if messages.IsAppAdded(message) || messages.IsAppWithdrawn(message) {
				var app App
				if messages.IsAppAdded(message) {
					name, address := messages.AppAddedDecode(message.BodyString)
					app = App{Name: name, Address: address}
				} else {
					app = App{Name: message.BodyString}
				}
				o.appsChan <- AppEvent{Type: message.Type, App: app}
			} else if messages.IsDisconnect(message) {
				break
			} else if messages.IsSessionOpened(message) || messages.IsSessionClosed(message) {
				o.sessionsChan <- message
			} else if messages.IsPing(message) {
				logrus.Tracef("Received ping message from %s", o.remoteName)
			} else {
				logrus.Warnf("Droping message of unknown type `%s`", message.Type)
			}
		}
		close(o.framesChan)
		close(o.appsChan)
		close(o.sessionsChan)
		close(o.closePingChan)
	}
}

func (o *DefaultPeer) startPinging() func() {
	return func() {
		defer func() {
			logrus.Debugf("Closing ping goroutine for peer %s", o.remoteName)
		}()
		timer := time.NewTicker(time.Second * 30)
		for {
			select {
			case <-timer.C:
				if pingErr := o.transport.Send(messages.NewPing()); pingErr != nil {
					if closeErr := o.Close(); closeErr != nil {
						logrus.Errorf(
							"Failed to send ping to peer %s - closing transport", o.remoteName,
						)
					}
					return
				}
			case <-o.closePingChan:
				return
			}
		}
	}
}

// NewDefaultPeer creates PeerConnection instances
func NewDefaultPeer(introduceAsName string, transport Transport) (*DefaultPeer, error) {
	theConnection := &DefaultPeer{
		transport:     transport,
		framesChan:    make(chan messages.Message),
		appsChan:      make(chan AppEvent),
		sessionsChan:  make(chan messages.Message),
		closePingChan: make(chan bool),
	}
	orchestrationFailed := make(chan error)
	grtn.Go(theConnection.startRouting(orchestrationFailed, introduceAsName))
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
	peersChan := make(chan Peer)
	grtn.Go(func() {
		for {
			transports, newTransportErr := defaultPeerFactory.transportFactory.Transports()
			if newTransportErr != nil {
				logrus.Error(fmt.Errorf("Error when creating transport: %w", newTransportErr))
				continue
			}
			for newTransport := range transports {
				newPeer, newPeerErr := NewDefaultPeer(defaultPeerFactory.ownName, newTransport)
				if newPeerErr != nil {
					logrus.Error(fmt.Errorf("Error when creating peer: %w", newPeerErr))
					continue
				}
				peersChan <- newPeer
			}
		}
	})
	return peersChan, nil
}

// NewDefaultPeerFactory creates PeerConnectionFactory instances
func NewDefaultPeerFactory(ownName string, transportFactory TransportFactory) PeerFactory {
	return &DefaultPeerFactory{transportFactory: transportFactory, ownName: ownName}
}
