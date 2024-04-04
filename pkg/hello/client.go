package hello

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/sirupsen/logrus"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Client struct {
	publicServerURL   string
	internalServerURL string
	name              string
	publicKey         string
	client            *http.Client

	currentWgConfig *wg.Config
	wgConfigWatcher *wg.Watcher

	apps         appRegistry
	syncInterval time.Duration

	remoteNginxAdapter *AppStateChangeGenerator
	remoteGatewayIP    string
}

func (c *Client) Hello() (string, error) {

	URL := c.publicServerURL + "/v1/hello"
	logrus.Infof("Registering as `%s` on server `%s`", c.name, URL)
	reqBodyJSON := helloRequest{
		Name:      c.name,
		PublicKey: c.publicKey,
	}
	reqBody, marshalErr := json.Marshal(reqBodyJSON)
	if marshalErr != nil {
		return "", fmt.Errorf("Failed to marshal request body: %v", marshalErr)
	}

	resp, err := c.client.Post(URL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("Failed to send request to server on URL %s: %v", URL, err)
	}
	bytes, readAllErr := io.ReadAll(resp.Body)
	if readAllErr != nil {
		return "", fmt.Errorf("Failed to read response body: %v", readAllErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Server returned status code %d on URL %s", resp.StatusCode, URL)
	}

	var respBody helloResponse
	if unmarshalErr := json.Unmarshal(bytes, &respBody); unmarshalErr != nil {
		return "", fmt.Errorf("Failed to unmarshal response body: %v", unmarshalErr)
	}
	c.currentWgConfig.Address = respBody.PeerIP
	c.currentWgConfig.Subnet = "24"
	peer := wg.Peer{
		Endpoint:   respBody.Peer.Endpoint,
		PublicKey:  respBody.Peer.PublicKey,
		AllowedIPs: fmt.Sprintf("%s/32", respBody.GatewayIP),
	}
	u, parseErr := url.Parse(c.publicServerURL)
	if parseErr != nil {
		return "", fmt.Errorf("Failed to parse URL %s: %v", c.publicServerURL, parseErr)
	}

	c.internalServerURL = fmt.Sprintf("http://%s:%s", respBody.GatewayIP, u.Port())
	logrus.WithFields(logrus.Fields{
		"gateway_ip": respBody.GatewayIP,
		"peer_ip":    respBody.PeerIP,
		"endpoint":   respBody.Peer.Endpoint,
	}).Info("Hello completed")
	c.currentWgConfig.Peers = []wg.Peer{peer}
	c.remoteGatewayIP = respBody.GatewayIP
	c.wgConfigWatcher.Update(*c.currentWgConfig)

	return respBody.GatewayIP, nil
}

func (c *Client) SyncForever() {
	for {
		if loopErr := func() error {
			URL := c.internalServerURL + "/v1/sync"

			apps := []syncRequestApp{}
			for _, app := range c.apps.Apps() {

				port, parseErr := strconv.Atoi(strings.Split(app.Address, ":")[1])
				if parseErr != nil {

					return fmt.Errorf("Failed to parse port: %v", parseErr)
				}
				apps = append(apps, syncRequestApp{
					Name:         app.Name,
					Peer:         app.Peer,
					Port:         port,
					OriginalPort: app.OriginalPort,
					TargetLabels: app.TargetLabels,
				})
			}
			reqBodyJSON := syncRequestAndResponse{
				Apps: apps,
			}
			reqBody, marshalErr := json.Marshal(reqBodyJSON)
			if marshalErr != nil {
				return fmt.Errorf("Failed to marshal request body: %v", marshalErr)
			}

			resp, err := c.client.Post(URL, "application/json", bytes.NewReader(reqBody))
			if err != nil {
				return fmt.Errorf("Failed to send request to server on URL %s: %v", URL, err)
			}
			bytes, readAllErr := io.ReadAll(resp.Body)
			if readAllErr != nil {
				return fmt.Errorf("Failed to read response body: %v", readAllErr)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("Server returned status code %d on URL %s", resp.StatusCode, URL)
			}

			var respBody syncRequestAndResponse
			if unmarshalErr := json.Unmarshal(bytes, &respBody); unmarshalErr != nil {
				return fmt.Errorf("Failed to unmarshal response body: %v", unmarshalErr)
			}
			c.remoteNginxAdapter.OnSync(
				"server",
				toPeerApps("server", c.remoteGatewayIP, respBody.Apps),
				nil,
			)
			return nil
		}(); loopErr != nil {
			logrus.Errorf("Failed to sync: %v", loopErr)
		}
		time.Sleep(c.syncInterval)
	}
}

func NewClient(serverURL, name string, apps appRegistry, remoteAdapter *AppStateChangeGenerator, wireguardWatcher *wg.Watcher) *Client {
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		logrus.Panicf("Failed to generate wireguard private key: %v", err)
	}
	cfg := &wg.Config{
		Address:    "10.188.1.1",
		PrivateKey: key.String(),
		Subnet:     "32",
	}
	return &Client{
		currentWgConfig: cfg,
		publicServerURL: serverURL,
		client: &http.Client{
			Timeout: time.Second * 5,
		},
		name:               name,
		publicKey:          key.PublicKey().String(),
		wgConfigWatcher:    wireguardWatcher,
		apps:               apps,
		syncInterval:       time.Second * 10,
		remoteNginxAdapter: remoteAdapter,
	}
}
