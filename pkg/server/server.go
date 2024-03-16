package server

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/ps"
	"github.com/glothriel/wormhole/pkg/router"
)

// Server accepts Peers and opens ports for all the apps connected peers expose
type Server struct {
	bus         ps.PubSub
	peerFactory peers.PeerFactory
	appExposer  AppExposer
}

func (server *Server) onAppAdded(peer peers.Peer, event peers.AppEvent) error {
	exposedApp, registerErr := server.appExposer.Expose(
		peer, event.App, router.NewPacketRouter(peer.Packets()),
	)
	if registerErr != nil {
		return registerErr
	}
	return peer.Send(
		messages.NewAppConfirmed(exposedApp.App.Name, exposedApp.App.Address),
	)
}
func (server *Server) onAppWithdrawn(peer peers.Peer, event peers.AppEvent) error {
	return server.appExposer.Unexpose(peer, event.App)
}

// Start launches the server
func (server *Server) Start() error {
	peersChan, peerErr := server.peerFactory.Peers()
	if peerErr != nil {
		return fmt.Errorf("Failed to start Peer factory %w", peerErr)
	}
	for peer := range peersChan {
		go func(peer peers.Peer) {

			OnRemoteAppExposed(server.bus, peer, func(ctx context.Context, app peers.App) error {
				pckts := make(chan messages.Message)
				exposedApp, registerErr := server.appExposer.Expose(
					peer, app, router.NewPacketRouter(pckts),
				)
				if registerErr != nil {
					return registerErr
				}
				OnLocalSessionStarted(server.bus, ".*", app.Name, peer.Name(), func(ctx context.Context, connection appConnection) error {
					handler := newAppConnectionHandler(
						peer,
						app,
						connection,
					)
					if sendErr := peer.Send(messages.WithContext(messages.NewSessionOpened(
						handler.appConnection.sessionID(),
						handler.app.Name,
					), ctx)); sendErr != nil {
						return sendErr
					}
					OnSessionAppData(server.bus, connection.sessionID(), app.Name, func(ctx context.Context, msg messages.Message) error {
						if messages.IsPacket(msg) {
							// TODO:This breaks at some point and blocks here
							pckts <- messages.WithContext(msg, ctx)

						}
						return nil
					})
					for {
						downstreamMsg, receiveErr := handler.appConnection.receive()
						if receiveErr != nil {
							if errors.Is(receiveErr, io.EOF) {
								if sessionClosedErr := handler.peer.Send(
									messages.WithContext(messages.NewSessionClosed(handler.appConnection.sessionID(), handler.app.Name), ctx),
								); sessionClosedErr != nil {
									return fmt.Errorf(
										"Failed to notify peer about closed session: %v", sessionClosedErr,
									)
								}
								return nil

							} else {
								return receiveErr
							}

						}

						if sendErr := handler.peer.Send(
							messages.WithContext(messages.WithAppName(downstreamMsg, handler.app.Name), ctx),
						); sendErr != nil {
							return sendErr
						}
					}
				})
				return peer.Send(
					messages.WithContext(messages.NewAppConfirmed(exposedApp.App.Name, exposedApp.App.Address), ctx),
				)
			})

			OnRemoteAppWithdrawn(server.bus, peer, func(ctx context.Context, app peers.App) error {
				unexposeErr := server.appExposer.Unexpose(peer, app)
				if unexposeErr != nil {
					return unexposeErr
				}
				return server.appExposer.Terminate(peer)
			})

		}(peer)
	}
	return nil
}

// NewServer creates Server instances
func NewServer(peerFactory peers.PeerFactory, appExposer AppExposer, bus ps.PubSub) *Server {
	listener := &Server{
		peerFactory: peerFactory,
		appExposer:  appExposer,
		bus:         bus,
	}
	return listener
}
