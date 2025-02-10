// Package cmd provides the command line interface for the wormhole application
package cmd

import (
	"time"

	"github.com/glothriel/wormhole/pkg/api"
	"github.com/glothriel/wormhole/pkg/k8s"
	"github.com/glothriel/wormhole/pkg/listeners"
	"github.com/glothriel/wormhole/pkg/nginx"
	"github.com/glothriel/wormhole/pkg/pairing"
	"github.com/glothriel/wormhole/pkg/syncing"
	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var helloRetryIntervalFlag *cli.DurationFlag = &cli.DurationFlag{
	Name:  "hello-retry-interval",
	Value: time.Second * 1,
}

var pairingServerURL *cli.StringFlag = &cli.StringFlag{
	Name:  "server",
	Value: "http://localhost:8080",
}

var clientCommand *cli.Command = &cli.Command{
	Name: "client",
	Flags: []cli.Flag{
		pairingServerURL,
		inviteTokenFlag,
		kubernetesFlag,
		stateManagerPathFlag,
		kubernetesNamespaceFlag,
		kubernetesLabelsFlag,
		peerNameFlag,
		enableNetworkPoliciesFlag,
		helloRetryIntervalFlag,
		nginxExposerConfdPathFlag,
		wireguardConfigFilePathFlag,
		pairingClientCacheDBPath,
		keyStorageDBFlag,
	},
	Action: func(c *cli.Context) error {
		privateKey, publicKey, keyErr := wg.GetOrGenerateKeyPair(getKeyStorage(c))
		if keyErr != nil {
			logrus.Fatalf("Failed to get key pair: %v", keyErr)
		}
		startPrometheusServer(c)

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
		remoteListenerRegistry := listeners.NewApps(effectiveExposer)

		appStateChangeGenerator := syncing.NewAppStateChangeGenerator()

		transport := pairing.NewHTTPClientPairingTransport(c.String(pairingServerURL.Name))
		if c.String(inviteTokenFlag.Name) != "" {
			transport = pairing.NewPSKClientPairingTransport(
				c.String(inviteTokenFlag.Name),
				transport,
			)
		}

		pairingKeyCache := pairing.NewInMemoryKeyCachingPairingClientStorage()
		if c.String(pairingClientCacheDBPath.Name) != "" {
			var err error
			pairingKeyCache, err = pairing.NewBoltKeyCachingPairingClientStorage(c.String(pairingClientCacheDBPath.Name))
			if err != nil {
				logrus.Fatalf("Failed to create pairing key cache: %v", err)
			}
		}
		wgReloader := wg.NewWatcher(c.String(wireguardConfigFilePathFlag.Name))
		wgConfig := &wg.Config{
			PrivateKey: privateKey,
			Subnet:     "32",
		}
		keyPair := pairing.KeyPair{
			PublicKey:  publicKey,
			PrivateKey: privateKey,
		}
		client := pairing.NewKeyCachingPairingClient(
			pairingKeyCache,
			wgConfig,
			wgReloader,
			pairing.NewDefaultPairingClient(
				c.String(peerNameFlag.Name),
				wgConfig,
				keyPair,
				wgReloader,
				pairing.NewJSONPairingEncoder(),
				transport,
			),
		)

		var pairingResponse pairing.Response
		for {
			var err error
			if pairingResponse, err = client.Pair(); err != nil {
				logrus.Error(err)
				time.Sleep(c.Duration(helloRetryIntervalFlag.Name))
				continue
			}
			break
		}
		localListenerRegistry := listeners.NewApps(nginx.NewNginxExposer(
			c.String(nginxExposerConfdPathFlag.Name),
			"local",
			nginx.NewDefaultReloader(),
			nginx.NewRangePortAllocator(20000, 25000),
			nginx.NewOnlyGivenAddressListener(pairingResponse.AssignedIP),
		))

		logrus.Infof("Paired with server, assigned IP: %s", pairingResponse.AssignedIP)
		go localListenerRegistry.Watch(getAppStateChangeGenerator(c).Changes(), make(chan bool))
		go remoteListenerRegistry.Watch(appStateChangeGenerator.Changes(), make(chan bool))

		sc, scErr := syncing.NewHTTPClient(
			c.String(peerNameFlag.Name),
			appStateChangeGenerator,
			syncing.NewJSONSyncingEncoder(),
			time.Second*5,
			syncing.NewAddressEnrichingAppSource(
				pairingResponse.AssignedIP,
				syncing.NewPeerEnrichingAppSource(
					c.String(peerNameFlag.Name),
					localListenerRegistry,
				),
			),
			pairingResponse,
			syncing.NewStaticMetadataFactory(syncing.Metadata{
				"foo": "bar",
			}),
		)
		if scErr != nil {
			logrus.Fatalf("Failed to create syncing client: %v", scErr)
		}

		go func() {
			err := api.NewAdminAPI([]api.Controller{
				api.NewAppsController(
					remoteListenerRegistry,
				),
			}, c.Bool(debugFlag.Name)).Run(":8082")
			if err != nil {
				logrus.Fatalf("Failed to start admin API: %v", err)
			}
		}()

		return sc.Start()
	},
}
