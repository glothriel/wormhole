package hello

import (
	"fmt"

	"github.com/glothriel/wormhole/pkg/wg"
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
