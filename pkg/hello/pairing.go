package hello

import (
	"fmt"

	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/sirupsen/logrus"
)

type PairingClient struct {
	clientName string
	keyPair    KeyPair
	wgConfig   *wg.Config

	wgReloader WireguardConfigReloader
	encoder    Marshaler
	transport  PairingClientTransport
}

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
		Endpoint:   decoded.Wireguard.Endpoint,
		PublicKey:  decoded.Wireguard.PublicKey,
		AllowedIPs: fmt.Sprintf("%s/32,%s/32", decoded.InternalServerIP, decoded.AssignedIP),
	})
	c.wgReloader.Update(*c.wgConfig)

	return decoded, nil

}

func NewPairingClient(
	clientName string,
	serverURL string,
	wgConfig *wg.Config,
	keyPair KeyPair,
	wgReloader WireguardConfigReloader,
	encoder Marshaler,
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

type MetadataEnricher interface {
	Metadata() map[string]string
}

type PairingServer struct {
	serverName       string     // Name of the server peer
	publicWgHostPort string     // Public Wireguard host:port, used in Endpoint field of the Wireguard config of other peers
	wgConfig         *wg.Config // Local Wireguard config
	keyPair          KeyPair    // Local Wireguard key pair

	wgReloader WireguardConfigReloader
	marshaler  Marshaler
	transport  PairingServerTransport
	ips        IPPool
	storage    PeerStorage
	enrichers  []MetadataEnricher
}

func (s *PairingServer) Start() {
	for incomingRequest := range s.transport.Requests() {
		logrus.Debugf("Received pairing request %v", incomingRequest)
		request, requestErr := s.marshaler.DecodeRequest(incomingRequest.Request)
		if requestErr != nil {
			incomingRequest.Err <- NewPairingRequestClientError(requestErr)
			continue
		}

		// Assign IP
		ip, ipErr := s.ips.Next()
		if ipErr != nil {
			incomingRequest.Err <- NewPairingRequestServerError(ipErr)
			continue
		}

		// Update local wireguard config
		s.wgConfig.Upsert(wg.Peer{
			PublicKey:  request.Wireguard.PublicKey,
			AllowedIPs: fmt.Sprintf("%s/32,%s/32", ip, s.wgConfig.Address),
		})
		s.wgReloader.Update(*s.wgConfig)

		// Store peer info
		storeErr := s.storage.Store(PeerInfo{
			Name:      request.Name,
			IP:        ip,
			PublicKey: request.Wireguard.PublicKey,
		})
		if storeErr != nil {
			incomingRequest.Err <- NewPairingRequestServerError(storeErr)
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

func NewPairingServer(
	serverName string,
	publicWgHostPort string,
	wgConfig *wg.Config,
	keyPair KeyPair,
	wgReloader WireguardConfigReloader,
	encoder Marshaler,
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
