package hello

import (
	"fmt"

	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/sirupsen/logrus"
)

// PairingClient is a client that can pair with a server
type PairingClient struct {
	clientName string
	keyPair    KeyPair
	wgConfig   *wg.Config

	wgReloader WireguardConfigReloader
	encoder    PairingEncoder
	transport  PairingClientTransport
}

// Pair sends a pairing request to the server and returns the response
func (c *PairingClient) Pair() (PairingResponse, error) {
	request := PairingRequest{
		Name: c.clientName,
		Wireguard: PairingRequestWireguardConfig{
			PublicKey: c.keyPair.PublicKey,
		},
		Metadata: map[string]string{},
	}
	encoded, encodeErr := c.encoder.EncodeRequest(request)
	if encodeErr != nil {
		return PairingResponse{}, NewPairingRequestClientError(encodeErr)
	}

	response, sendErr := c.transport.Send(encoded)
	if sendErr != nil {
		return PairingResponse{}, NewPairingRequestClientError(sendErr)
	}

	decoded, decodeErr := c.encoder.DecodeResponse(response)
	if decodeErr != nil {
		return PairingResponse{}, NewPairingRequestClientError(decodeErr)
	}
	c.wgConfig.Address = decoded.AssignedIP
	c.wgConfig.Upsert(wg.Peer{
		Endpoint:            decoded.Wireguard.Endpoint,
		PublicKey:           decoded.Wireguard.PublicKey,
		AllowedIPs:          fmt.Sprintf("%s/32,%s/32", decoded.InternalServerIP, decoded.AssignedIP),
		PersistentKeepalive: 10,
	})

	return decoded, c.wgReloader.Update(*c.wgConfig)
}

// NewPairingClient creates a new PairingClient instance
func NewPairingClient(
	clientName string,
	wgConfig *wg.Config,
	keyPair KeyPair,
	wgReloader WireguardConfigReloader,
	encoder PairingEncoder,
	transport PairingClientTransport,
) *PairingClient {
	return &PairingClient{
		clientName: clientName,
		keyPair:    keyPair,
		wgConfig:   wgConfig,
		wgReloader: wgReloader,
		encoder:    encoder,
		transport:  transport,
	}
}

// MetadataEnricher is an interface that allows transports exchanging information between
// their client/server implementations
type MetadataEnricher interface {
	Metadata() map[string]string
}

// PairingServer is a server that can pair with multiple clients
type PairingServer struct {
	serverName       string     // Name of the server peer
	publicWgHostPort string     // Public Wireguard host:port
	wgConfig         *wg.Config // Local Wireguard config
	keyPair          KeyPair    // Local Wireguard key pair

	wgReloader WireguardConfigReloader
	marshaler  PairingEncoder
	transport  PairingServerTransport
	ips        IPPool
	storage    PeerStorage
	enrichers  []MetadataEnricher
}

// Start starts the pairing server
func (s *PairingServer) Start() { // nolint: funlen, gocognit
	for incomingRequest := range s.transport.Requests() {
		request, requestErr := s.marshaler.DecodeRequest(incomingRequest.Request)
		if requestErr != nil {
			incomingRequest.Err <- NewPairingRequestClientError(requestErr)
			continue
		}

		var ip string
		var publicKey string
		existingPeer, peerErr := s.storage.GetByName(request.Name)
		if peerErr != nil {
			if peerErr != ErrPeerDoesNotExist {
				incomingRequest.Err <- NewPairingRequestServerError(peerErr)
				continue
			}
			// Peer is not in the Database
			var ipErr error
			ip, ipErr = s.ips.Next()
			if ipErr != nil {
				incomingRequest.Err <- NewPairingRequestServerError(ipErr)
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
				incomingRequest.Err <- NewPairingRequestServerError(storeErr)
				continue
			}
		} else {
			if existingPeer.PublicKey != request.Wireguard.PublicKey {
				logrus.Errorf(
					"attempted peering from peer %s: error, public key mismatch", request.Name,
				)
				continue
			}
			// Peer is in the Database
			ip = existingPeer.IP
			publicKey = existingPeer.PublicKey
		}

		// Update local wireguard config
		s.wgConfig.Upsert(wg.Peer{
			PublicKey:  publicKey,
			AllowedIPs: fmt.Sprintf("%s/32,%s/32", ip, s.wgConfig.Address),
		})
		wgUpdateErr := s.wgReloader.Update(*s.wgConfig)
		if wgUpdateErr != nil {
			incomingRequest.Err <- NewPairingRequestServerError(wgUpdateErr)
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
		response := PairingResponse{
			Name:             s.serverName,
			AssignedIP:       ip,
			InternalServerIP: s.wgConfig.Address,
			Wireguard: PairingResponseWireguardConfig{
				PublicKey: s.keyPair.PublicKey,
				Endpoint:  s.publicWgHostPort,
			},
			Metadata: metadata,
		}
		encoded, encodeErr := s.marshaler.EncodeResponse(response)
		if encodeErr != nil {
			incomingRequest.Err <- NewPairingRequestServerError(encodeErr)
			continue
		}
		logrus.Infof("Pairing request from %s, assigned IP %s", request.Name, response.AssignedIP)
		incomingRequest.Response <- encoded
	}
}

// NewPairingServer creates a new PairingServer instance
func NewPairingServer(
	serverName string,
	publicWgHostPort string,
	wgConfig *wg.Config,
	keyPair KeyPair,
	wgReloader WireguardConfigReloader,
	encoder PairingEncoder,
	transport PairingServerTransport,
	ips IPPool,
	storage PeerStorage,
	enrichers []MetadataEnricher,
) *PairingServer {
	return &PairingServer{
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
