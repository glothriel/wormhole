package client

import (
	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
)

// Exposer exposes given apps via the peer
type Exposer struct {
	Peer peers.Peer
}

// Expose connects to the peer and instructs it to expose the apps
func (e *Exposer) Expose(apps ...peers.App) error {
	appsRegistry := newAppConnectionRegistry()
	upstreamAddressesByName := map[string]string{}
	for _, upstreamDefinition := range apps {
		if sendErr := e.Peer.Send(messages.NewAppAdded(upstreamDefinition.Name)); sendErr != nil {
			return sendErr
		}
		upstreamAddressesByName[upstreamDefinition.Name] = upstreamDefinition.Address
	}

	msgs, receiveErr := e.Peer.Receive()
	if receiveErr != nil {
		return receiveErr
	}
	for theMsg := range msgs {
		if messages.IsPing(theMsg) {
			continue
		}
		upstreamConnection, upstreamConnectionErr := appsRegistry.getOrCreateForID(
			theMsg.SessionID,
			upstreamAddressesByName[theMsg.AppName],
			theMsg.AppName,
			e.Peer,
		)
		if upstreamConnectionErr != nil {
			logrus.Errorf("Get or create upstream connection error: %v", upstreamConnectionErr)
			continue
		}

		if messages.IsFrame(theMsg) {
			upstreamConnection.outbox() <- theMsg
		}
		if messages.IsDisconnect(theMsg) {
			upstreamConnection, upstreamConnectionErr := appsRegistry.getForID(theMsg.SessionID)
			if upstreamConnectionErr != nil {
				logrus.Errorf("Get upstream connection error: %v", upstreamConnectionErr)
			}
			upstreamConnection.terminate()
		}
	}
	return nil
}

// NewExposer creates Exposer instances
func NewExposer(peer peers.Peer) *Exposer {
	return &Exposer{
		Peer: peer,
	}
}
