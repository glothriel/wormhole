package pairing

import (
	"errors"
	"fmt"

	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/sirupsen/logrus"
)

// Server is a server that can pair with multiple clients
type Server struct {
	serverName       string     // Name of the server peer
	publicWgHostPort string     // Public Wireguard host:port
	wgConfig         *wg.Config // Local Wireguard config
	keyPair          KeyPair    // Local Wireguard key pair

	wgReloader wg.WireguardConfigReloader
	marshaler  Encoder
	transport  ServerTransport
	ips        IPPool
	storage    PeerStorage
	enrichers  []MetadataEnricher
}

// Start starts the pairing server
func (s *Server) Start() { // nolint: funlen, gocognit
	for incomingRequest := range s.transport.Requests() {
		request, requestErr := s.marshaler.DecodeRequest(incomingRequest.Request)
		if requestErr != nil {
			incomingRequest.Err <- NewClientError(requestErr)
			continue
		}

		var ip string
		var publicKey string
		existingPeer, peerErr := s.storage.GetByName(request.Name)
		if peerErr != nil {
			if peerErr != ErrPeerDoesNotExist {
				incomingRequest.Err <- NewServerError(peerErr)
				continue
			}
			// Peer is not in the Database
			var ipErr error
			ip, ipErr = s.ips.Next()
			if ipErr != nil {
				incomingRequest.Err <- NewServerError(ipErr)
				continue
			}
			publicKey = request.Wireguard.PublicKey

			// Store peer info
			storeErr := s.storage.Store(PeerInfo{
				Name:      request.Name,
				IP:        ip,
				PublicKey: publicKey,
			})

			if storeErr != nil {
				incomingRequest.Err <- NewServerError(storeErr)
				continue
			}
		} else {
			if existingPeer.PublicKey != request.Wireguard.PublicKey {
				logrus.Errorf(
					"attempted peering from peer `%s`: error, public key mismatch. "+
						"There's existing peer `%s` with a different public key.",
					request.Name, existingPeer.Name,
				)
				incomingRequest.Err <- NewServerError(
					errors.New("please see the server log for error details"),
				)
				continue
			}
			// Peer is in the Database
			ip = existingPeer.IP
			publicKey = existingPeer.PublicKey
		}

		// Update local wireguard config
		s.wgConfig.Upsert(wg.Peer{
			Name:       request.Name,
			PublicKey:  publicKey,
			AllowedIPs: fmt.Sprintf("%s/32,%s/32", ip, s.wgConfig.Address),
		})
		wgUpdateErr := s.wgReloader.Update(*s.wgConfig)
		if wgUpdateErr != nil {
			incomingRequest.Err <- NewServerError(wgUpdateErr)
			continue
		}

		// Enrich metadata
		metadata := map[string]string{}
		for _, enricher := range s.enrichers {
			for k, v := range enricher.Metadata() {
				metadata[k] = v
			}
		}

		// Respond to the client
		response := Response{
			Name:             s.serverName,
			AssignedIP:       ip,
			InternalServerIP: s.wgConfig.Address,
			Wireguard: ResponseWireguardConfig{
				PublicKey: s.keyPair.PublicKey,
				Endpoint:  s.publicWgHostPort,
			},
			Metadata: metadata,
		}
		encoded, encodeErr := s.marshaler.EncodeResponse(response)
		if encodeErr != nil {
			incomingRequest.Err <- NewServerError(encodeErr)
			continue
		}
		logrus.Infof("Pairing request from %s, assigned IP %s", request.Name, response.AssignedIP)
		incomingRequest.Response <- encoded
	}
}

// NewServer creates a new PairingServer instance
func NewServer(
	serverName string,
	publicWgHostPort string,
	wgConfig *wg.Config,
	keyPair KeyPair,
	wgReloader wg.WireguardConfigReloader,
	encoder Encoder,
	transport ServerTransport,
	ips IPPool,
	storage PeerStorage,
	enrichers []MetadataEnricher,
) *Server {
	return &Server{
		serverName:       serverName,
		publicWgHostPort: publicWgHostPort,
		wgConfig:         wgConfig,
		keyPair:          keyPair,
		wgReloader:       wgReloader,
		marshaler:        encoder,
		transport:        transport,
		ips:              ips,
		storage:          storage,
		enrichers:        enrichers,
	}
}

// MetadataEnricher is an interface that allows transports exchanging information between
// their client/server implementations
type MetadataEnricher interface {
	Metadata() map[string]string
}
