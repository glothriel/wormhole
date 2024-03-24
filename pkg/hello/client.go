package hello

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/sirupsen/logrus"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Client struct {
	serverURL     string
	name          string
	publicKey     string
	cfg           *wg.Cfg
	client        *http.Client
	configWatcher *wg.Watcher
}

func (c *Client) Hello() (string, error) {

	getUrl := c.serverURL + "/v1/hello"
	reqBodyJSON := helloRequest{
		Name:      c.name,
		PublicKey: c.publicKey,
	}
	reqBody, marshalErr := json.Marshal(reqBodyJSON)
	if marshalErr != nil {
		return "", fmt.Errorf("Failed to marshal request body: %v", marshalErr)
	}

	resp, err := c.client.Post(getUrl, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("Failed to send request to server on URL %s: %v", getUrl, err)
	}
	bytes, readAllErr := io.ReadAll(resp.Body)
	if readAllErr != nil {
		return "", fmt.Errorf("Failed to read response body: %v", readAllErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Server returned status code %d on URL %s", resp.StatusCode, getUrl)
	}

	var respBody helloResponse
	if unmarshalErr := json.Unmarshal(bytes, &respBody); unmarshalErr != nil {
		return "", fmt.Errorf("Failed to unmarshal response body: %v", unmarshalErr)
	}
	c.cfg.Address = respBody.PeerIP
	c.cfg.Subnet = "24"
	peer := wg.Peer{
		Endpoint:   respBody.Peer.Endpoint,
		PublicKey:  respBody.Peer.PublicKey,
		AllowedIPs: fmt.Sprintf("%s/32", respBody.GatewayIP),
	}
	c.cfg.Peers = []wg.Peer{peer}

	c.configWatcher.Update(*c.cfg)

	return resp.Status, nil
}

func NewClient(serverURL, name string) *Client {
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		logrus.Fatalf("Failed to generate wireguard private key: %v", err)
	}
	cfg := &wg.Cfg{
		Address:    "10.188.1.1",
		PrivateKey: key.String(),
		Subnet:     "32",
	}
	return &Client{
		cfg:       cfg,
		serverURL: serverURL,
		client: &http.Client{
			Timeout: time.Second * 5,
		},
		publicKey:     key.PublicKey().String(),
		configWatcher: wg.NewWriter("/storage/wireguard/wg0.conf"),
	}
}
