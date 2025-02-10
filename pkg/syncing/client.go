package syncing

import (
	"errors"
	"fmt"
	"time"

	"github.com/glothriel/wormhole/pkg/pairing"
	"github.com/sirupsen/logrus"
)

// Client is a struct that orchestrates all the operations that are performed client-side
// when executing app list synchronizations
type Client struct {
	myName               string
	stateChangeGenerator *AppStateChangeGenerator
	encoder              Encoder
	interval             time.Duration
	apps                 AppSource
	transport            ClientTransport
	failureThreshold     int
	metadata             MetadataFactory
}

// Start starts the syncing client
func (c *Client) Start() error {
	failures := 0
	for {
		time.Sleep(c.interval)
		apps, listErr := c.apps.List()
		if listErr != nil {
			logrus.Errorf("failed to list apps: %v", listErr)
			continue
		}
		metadata, metadataErr := c.metadata.Get()
		if metadataErr != nil {
			logrus.Errorf("failed to get metadata: %v", metadataErr)
			continue
		}
		encodedApps, encodeErr := c.encoder.Encode(Message{
			Peer:     c.myName,
			Metadata: metadata,
			Apps:     apps,
		})
		if encodeErr != nil {
			logrus.Errorf("failed to encode apps: %v", encodeErr)
			continue
		}
		incomingApps, err := c.transport.Sync(encodedApps)
		if err != nil {
			if failures >= c.failureThreshold {
				return fmt.Errorf("fatal: failed to sync %d times in a row: %v", failures, err)
			}
			failures++
			logrus.Errorf("failed to sync apps: %v", err)
			continue
		}
		failures = 0
		decodedMsg, decodeErr := c.encoder.Decode(incomingApps)
		if decodeErr != nil {
			logrus.Errorf("failed to decode incoming apps: %v", decodeErr)
			continue
		}
		c.stateChangeGenerator.UpdateForPeer(
			decodedMsg.Peer,
			decodedMsg.Apps,
		)
	}
}

// NewClient creates a new SyncingClient instance
func NewClient(
	myName string,
	nginxAdapter *AppStateChangeGenerator,
	encoder Encoder,
	interval time.Duration,
	apps AppSource,
	transport ClientTransport,
	MetadataFactory MetadataFactory,
) *Client {
	return &Client{
		myName:               myName,
		stateChangeGenerator: nginxAdapter,
		encoder:              encoder,
		interval:             interval,
		apps:                 apps,
		transport:            transport,
		failureThreshold:     3,
		metadata:             MetadataFactory,
	}
}

// NewHTTPClient creates a new SyncingClient instance with HTTP transport
func NewHTTPClient(
	myName string,
	nginxAdapter *AppStateChangeGenerator,
	encoder Encoder,
	interval time.Duration,
	apps AppSource,
	pr pairing.Response,
	metadata MetadataFactory,

) (*Client, error) {
	syncServerAddress, ok := pr.Metadata["sync_server_address"]
	if !ok {
		return nil, errors.New("sync_server_address not found in pairing response metadata")
	}
	transport := NewHTTPClientTransport(syncServerAddress, 3*time.Second)
	return NewClient(
		myName,
		nginxAdapter,
		encoder,
		interval,
		apps,
		transport,
		metadata,
	), nil
}
