package peers

import (
	"context"
	"fmt"
	"time"

	"github.com/glothriel/wormhole/pkg/events"
	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/ps"
	"github.com/sirupsen/logrus"
)

// DefaultPeer implements Peer by plucing out Transport layer into another interface
type DefaultPeer struct {
	remoteName string

	transport     Transport
	packetsChan   chan messages.Message
	sessionsChan  chan messages.Message
	appsChan      chan AppEvent
	closePingChan chan bool
	pubSub        ps.PubSub
}

// Name implements Peer
func (o *DefaultPeer) Name() string {
	return o.remoteName
}

// Packets returns messages that are used to interchange app data
func (o *DefaultPeer) Packets() chan messages.Message {
	return o.packetsChan
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
	logrus.Debug("[PEER] Sent message: ", msg.Type)
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

	if sendErr := o.Send(messages.NewIntroduction(localName)); sendErr != nil {
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
	go o.startPinging()
	for message := range messagesChan {
		ctx := messages.ParseContext(message)

		logrus.Debug("[PEER] Received message: ", message.Type)

		if messages.IsPacketFromClient(message) {
			o.pubSub.Publish(events.RemoteSessionClientDataSentTopic(message.SessionID, message.AppName), ctx, message)
		} else if messages.IsPacketFromApp(message) {
			o.pubSub.Publish(events.RemoteSessionAppDataSentTopic(message.SessionID, message.AppName), ctx, message)
		} else if messages.IsAppAdded(message) || messages.IsAppWithdrawn(message) {

			if messages.IsAppAdded(message) {
				name, address := messages.AppEventsDecode(message.BodyString)
				o.pubSub.Publish(events.RemoteAppExposedTopic(o.Name()), ctx, App{Name: name, Address: address})
			} else {
				o.pubSub.Publish(events.RemoteAppExposedTopic(o.Name()), ctx, App{Name: message.BodyString})
			}

		} else if messages.IsDisconnect(message) {
			break
		} else if messages.IsSessionOpened(message) {
			o.pubSub.Publish(events.RemoteSessionStartedTopic(message.SessionID), ctx, message)
		} else if messages.IsSessionClosed(message) {
			o.pubSub.Publish(events.RemoteSessionFinishedTopic(message.SessionID), ctx, message)
		} else if messages.IsPing(message) {
			o.pubSub.Publish(events.PingTopic, ctx, message)
		} else if messages.IsAppConfirmed(message) {
			name, address := messages.AppEventsDecode(message.BodyString)
			logrus.Infof("Server confirmed app `%s` exposed on `%s`", name, address)
		} else {
			logrus.Warnf("Dropping message of unknown type `%s`", message.Type)
		}
	}
	close(o.packetsChan)
	close(o.appsChan)
	close(o.sessionsChan)
	close(o.closePingChan)
}

func (o *DefaultPeer) startPinging() {
	defer func() {
		logrus.Debugf("Closing ping goroutine for peer %s", o.remoteName)
	}()
	timer := time.NewTicker(time.Second * 30)
	for {
		select {
		case <-timer.C:
			if pingErr := o.Send(messages.NewPing()); pingErr != nil {
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

// NewDefaultPeer creates PeerConnection instances
func NewDefaultPeer(introduceAsName string, transport Transport, pubSub ps.PubSub) (*DefaultPeer, error) {
	theConnection := &DefaultPeer{
		transport:     transport,
		packetsChan:   make(chan messages.Message),
		appsChan:      make(chan AppEvent),
		sessionsChan:  make(chan messages.Message),
		closePingChan: make(chan bool),
		pubSub:        pubSub,
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
	pubsSub          ps.PubSub
}

// Peers implements PeerFactory
func (defaultPeerFactory *DefaultPeerFactory) Peers() (chan Peer, error) {
	peersChan := make(chan Peer)
	go func() {
		for {
			transports, newTransportErr := defaultPeerFactory.transportFactory.Transports()
			if newTransportErr != nil {
				logrus.Error(fmt.Errorf("Error when creating transport: %w", newTransportErr))
				continue
			}
			for newTransport := range transports {
				newPeer, newPeerErr := NewDefaultPeer(defaultPeerFactory.ownName, newTransport, defaultPeerFactory.pubsSub)
				if newPeerErr != nil {
					logrus.Error(fmt.Errorf("Error when creating peer: %w", newPeerErr))
					continue
				}
				defaultPeerFactory.pubsSub.Publish(events.PeerConnected, context.Background(), newPeer)
				peersChan <- newPeer
			}
		}
	}()
	return peersChan, nil
}

// NewDefaultPeerFactory creates PeerConnectionFactory instances
func NewDefaultPeerFactory(ownName string, transportFactory TransportFactory, pubSub ps.PubSub) PeerFactory {
	return &DefaultPeerFactory{transportFactory: transportFactory, ownName: ownName, pubsSub: pubSub}
}
