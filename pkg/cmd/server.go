package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/glothriel/wormhole/pkg/api"
	"github.com/glothriel/wormhole/pkg/pairing"
	"github.com/glothriel/wormhole/pkg/syncing"

	"github.com/glothriel/wormhole/pkg/k8s"
	"github.com/glothriel/wormhole/pkg/listeners"
	"github.com/glothriel/wormhole/pkg/nginx"
	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	wgAddressFlag *cli.StringFlag = &cli.StringFlag{
		Name:     "wg-internal-host",
		Required: true,
	}

	wgSubnetFlag *cli.StringFlag = &cli.StringFlag{
		Name:  "wg-subnet-mask",
		Value: "24",
	}

	wgPublicHostFlag *cli.StringFlag = &cli.StringFlag{
		Name:     "wg-public-host",
		Required: true,
	}

	wgPortFlag *cli.IntFlag = &cli.IntFlag{
		Name:  "wg-port",
		Value: 51820,
	}

	extServerListenAddress *cli.StringFlag = &cli.StringFlag{
		Name:  "ext-server-listen-address",
		Value: "0.0.0.0:8080",
	}

	intServerListenPort *cli.IntFlag = &cli.IntFlag{
		Name:  "int-server-listen-port",
		Value: 8081,
	}
)

var serverCommand *cli.Command = &cli.Command{
	Name: "server",
	Flags: []cli.Flag{
		kubernetesFlag,
		inviteTokenFlag,
		stateManagerPathFlag,
		nginxExposerConfdPathFlag,
		wgPublicHostFlag,
		wireguardConfigFilePathFlag,
		extServerListenAddress,
		intServerListenPort,
		kubernetesNamespaceFlag,
		kubernetesLabelsFlag,
		enableNetworkPoliciesFlag,
		peerStorageDBFlag,
		peerControllerEnableDeletionFlag,
		peerNameFlag,
		wgAddressFlag,
		wgSubnetFlag,
		wgPortFlag,
		keyStorageDBFlag,
	},
	Action: func(c *cli.Context) error {
		startPrometheusServer(c)

		privateKey, publicKey, keyErr := wg.GetOrGenerateKeyPair(getKeyStorage(c))
		if keyErr != nil {
			logrus.Fatalf("Failed to get or generate key pair: %v", keyErr)
		}

		appsExposedHere := listeners.NewApps(nginx.NewNginxExposer(
			c.String(nginxExposerConfdPathFlag.Name),
			"local",
			nginx.NewDefaultReloader(),
			nginx.NewRangePortAllocator(20000, 25000),
			nginx.NewOnlyGivenAddressListener(c.String(wgAddressFlag.Name)),
		))

		remoteNginxExposer := nginx.NewNginxExposer(
			c.String(nginxExposerConfdPathFlag.Name),
			"remote",
			nginx.NewDefaultReloader(),
			nginx.NewRangePortAllocator(25001, 30000),
			nginx.NewAllAcceptWireguardListener(),
		)
		var effectiveExposer listeners.Exposer = remoteNginxExposer

		if c.Bool(kubernetesFlag.Name) {
			namespace := c.String(kubernetesNamespaceFlag.Name)
			rawLabels := c.String(kubernetesLabelsFlag.Name)
			if namespace == "" || rawLabels == "" {
				logrus.Fatalf(
					"Namespace (--%s) and labels (--%s) must be set when using kubernetes integration",
					kubernetesNamespaceFlag.Name,
					kubernetesLabelsFlag.Name,
				)
			}
			effectiveExposer = k8s.NewK8sExposer(
				c.String(kubernetesNamespaceFlag.Name),
				k8s.CSVToMap(c.String(kubernetesLabelsFlag.Name)),
				c.Bool(enableNetworkPoliciesFlag.Name),
				remoteNginxExposer,
			)
		}
		appsExposedFromRemote := listeners.NewApps(effectiveExposer)

		go appsExposedHere.Watch(getAppStateChangeGenerator(c).Changes(), make(chan bool))

		remoteNginxAdapter := syncing.NewAppStateChangeGenerator()
		go appsExposedFromRemote.Watch(remoteNginxAdapter.Changes(), make(chan bool))

		wgConfig := &wg.Config{
			Address:    c.String(wgAddressFlag.Name),
			Subnet:     c.String(wgSubnetFlag.Name),
			ListenPort: c.Int(wgPortFlag.Name),
			PrivateKey: privateKey,
		}
		peerStorage := getPeerStorage(c)
		savedPeers, peersErr := peerStorage.List()
		if peersErr != nil {
			logrus.Panicf("failed to list peers: %v", peersErr)
		}
		for _, savedPeer := range savedPeers {
			wgConfig.Upsert(wg.Peer{
				Name:       savedPeer.Name,
				PublicKey:  savedPeer.PublicKey,
				AllowedIPs: fmt.Sprintf("%s/32,%s/32", savedPeer.IP, wgConfig.Address),
			})
		}
		syncTransport := syncing.NewHTTPServerSyncingTransport(&http.Server{
			Addr:              fmt.Sprintf("%s:%d", c.String(wgAddressFlag.Name), c.Int(intServerListenPort.Name)),
			ReadHeaderTimeout: time.Second * 5,
		})

		appSource := syncing.NewAddressEnrichingAppSource(
			wgConfig.Address,
			syncing.NewPeerEnrichingAppSource("server", appsExposedHere),
		)

		metadataStorage := syncing.NewInMemoryMetadataStorage()

		ss := syncing.NewServer(
			c.String(peerNameFlag.Name),
			remoteNginxAdapter,
			appSource,
			syncing.NewJSONSyncingEncoder(),
			syncTransport,
			peerStorage,
			metadataStorage,
		)
		watcher := wg.NewWatcher(c.String(wireguardConfigFilePathFlag.Name))
		updateErr := watcher.Update(*wgConfig)
		if updateErr != nil {
			return fmt.Errorf("failed to bootstrap wireguard config: %w", updateErr)
		}
		peerTransport := pairing.NewHTTPServerPairingTransport(&http.Server{
			Addr:              c.String(extServerListenAddress.Name),
			ReadHeaderTimeout: time.Second * 5,
		})
		if c.String(inviteTokenFlag.Name) != "" {
			peerTransport = pairing.NewPSKPairingServerTransport(
				c.String(inviteTokenFlag.Name),
				peerTransport,
			)
		}
		ps := pairing.NewServer(
			"server",
			fmt.Sprintf("%s:%d", c.String(wgPublicHostFlag.Name), c.Int(wgPortFlag.Name)),
			wgConfig,
			pairing.KeyPair{
				PublicKey:  publicKey,
				PrivateKey: privateKey,
			},
			watcher,
			pairing.NewJSONPairingEncoder(),
			peerTransport,
			pairing.NewIPPool(c.String(wgAddressFlag.Name), pairing.NewReservedAddressLister(
				peerStorage,
			)),
			peerStorage,
			[]pairing.MetadataEnricher{syncTransport},
		)
		go ss.Start()
		go func() {
			peerControllerSettings := []api.PeerControllerSettings{}
			if c.Bool(peerControllerEnableDeletionFlag.Name) {
				peerControllerSettings = append(peerControllerSettings, api.EnablePeerDeletion)
			}
			err := api.NewAdminAPI([]api.Controller{
				api.NewAppsController(appsExposedFromRemote),
				api.NewPeersController(peerStorage, wgConfig, watcher, metadataStorage, peerControllerSettings...),
			}, c.Bool(debugFlag.Name)).Run(":8082")
			if err != nil {
				logrus.Fatalf("Failed to start admin API: %v", err)
			}
		}()

		ps.Start()
		return nil
	},
}
