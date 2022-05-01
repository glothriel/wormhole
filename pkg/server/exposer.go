package server

import (
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
)

// AppExposer is responsible for keeping track of which apps are registered and their endpoints exported
type AppExposer interface {
	Expose(peer peers.Peer, app peers.App, router messageRouter) error
	Unexpose(peer peers.Peer, app peers.App) error
	Apps() []ExposedApp
	Terminate(peer peers.Peer) error
}

// ExposedApp represents an app exposed on the server along with the peer the app is exposed from
type ExposedApp struct {
	App  peers.App
	Peer peers.Peer
}
type defaultAppExposer struct {
	registry *exposedAppsRegistry

	portOpenerFactory PortOpenerFactory
}

type portOpener interface {
	connections() chan appConnection
	listenAddr() string
	close() error
}

// PortOpenerFactory is a factory interface for portOpener
type PortOpenerFactory interface {
	Create(app peers.App, peer peers.Peer) (portOpener, error)
}

func (exposer *defaultAppExposer) Expose(peer peers.Peer, app peers.App, router messageRouter) error {
	portOpener, portOpenerErr := exposer.portOpenerFactory.Create(app, peer)
	if portOpenerErr != nil {
		return portOpenerErr
	}
	app.Address = portOpener.listenAddr()

	logrus.Infof("App `%s`.`%s`: listening on %s", peer.Name(), app.Name, portOpener.listenAddr())
	exposer.registry.store(peer, app, portOpener)
	go func() {
		for connection := range portOpener.connections() {
			handler := newAppConnectionHandler(
				peer,
				app,
				connection,
			)
			go handler.Handle(router)
		}
		exposer.registry.delete(peer, app)
	}()
	return nil
}

func (exposer *defaultAppExposer) Unexpose(peer peers.Peer, app peers.App) error {
	portOpener, found := exposer.registry.get(peer, app)
	if !found {
		return nil
	}
	if closeErr := portOpener.close(); closeErr != nil {
		return closeErr
	}
	exposer.registry.delete(peer, app)
	return nil
}

func (exposer *defaultAppExposer) Terminate(peer peers.Peer) error {
	for _, storedExposerEntry := range exposer.registry.items() {
		if storedExposerEntry.peer.Name() == peer.Name() {
			if closeErr := storedExposerEntry.portOpener.close(); closeErr != nil {
				logrus.Warnf("Could not close port exposer: %v", closeErr)
				continue
			}
		}
	}
	exposer.registry.deleteAll(peer)
	return nil
}

func (exposer *defaultAppExposer) Apps() []ExposedApp {
	allApps := []ExposedApp{}
	for _, storedExposerEntry := range exposer.registry.items() {
		allApps = append(allApps, ExposedApp{
			App:  storedExposerEntry.app,
			Peer: storedExposerEntry.peer,
		})
	}
	return allApps
}

// NewDefaultAppExposer creates defaultAppExposer instances
func NewDefaultAppExposer(portOpenerFactory PortOpenerFactory) AppExposer {
	return &defaultAppExposer{
		portOpenerFactory: portOpenerFactory,
		registry:          newExposedAppsRegistry(),
	}
}
