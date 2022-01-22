package client

import (
	"fmt"
	"sync"

	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
)

type appConnectionsRegistry struct {
	upstreamConnections sync.Map
}

func (registry *appConnectionsRegistry) getOrCreateForID(
	sessionID, destination, upstreamName string, peer peers.Peer,
) (*appConnection, error) {
	session, found := registry.upstreamConnections.Load(sessionID)
	if !found {
		logrus.WithField("session_id", sessionID).Infof("Creating new client session on %s", destination)
		theSession, sessionErr := newAppConnection(sessionID, destination, upstreamName, peer)
		if sessionErr != nil {
			return nil, sessionErr
		}
		registry.upstreamConnections.Store(sessionID, theSession)
		return theSession, nil
	}
	return session.(*appConnection), nil
}

func (registry *appConnectionsRegistry) getForID(sessionID string) (*appConnection, error) {
	session, found := registry.upstreamConnections.Load(sessionID)
	if !found {
		return nil, fmt.Errorf("Could not find session with ID %s", sessionID)
	}
	return session.(*appConnection), nil
}

func newAppConnectionRegistry() *appConnectionsRegistry {
	return &appConnectionsRegistry{
		upstreamConnections: sync.Map{},
	}
}
