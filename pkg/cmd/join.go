package cmd

import (
	"time"

	"github.com/glothriel/wormhole/pkg/hello"
	"github.com/glothriel/wormhole/pkg/k8s"
	"github.com/glothriel/wormhole/pkg/listeners"
	"github.com/glothriel/wormhole/pkg/nginx"
	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var helloRetryIntervalFlag *cli.DurationFlag = &cli.DurationFlag{
	Name:  "hello-retry-interval",
	Value: time.Second * 1,
}

var peerNameFlag *cli.StringFlag = &cli.StringFlag{
	Name:     "name",
	Required: true,
}

var serverUrlFlag *cli.StringFlag = &cli.StringFlag{
	Name:  "server",
	Value: "http://localhost:8080",
}

var joinCommand *cli.Command = &cli.Command{
	Name: "join",
	Flags: []cli.Flag{
		serverUrlFlag,
		kubernetesFlag,
		stateManagerPathFlag,
		kubernetesNamespaceFlag,
		kubernetesLabelsFlag,
		peerNameFlag,
		helloRetryIntervalFlag,
		nginxExposerConfdPathFlag,
		wireguardConfigFilePathFlag,
	},
	Action: func(c *cli.Context) error {
		pkey, keyErr := wgtypes.GeneratePrivateKey()
		if keyErr != nil {
			logrus.Fatalf("Failed to generate private key: %v", keyErr)
		}
		startPrometheusServer(c)

		localListenerRegistry := listeners.NewRegistry(nginx.NewNginxExposer(
			c.String(nginxExposerConfdPathFlag.Name),
			"local",
			nginx.NewDefaultReloader(),
			nginx.NewRangePortAllocator(20000, 25000),
		))

		remoteNginxExposer := nginx.NewNginxExposer(
			c.String(nginxExposerConfdPathFlag.Name),
			"remote",
			nginx.NewDefaultReloader(),
			nginx.NewRangePortAllocator(25001, 30000),
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
				remoteNginxExposer,
			)
		}
		remoteListenerRegistry := listeners.NewRegistry(effectiveExposer)

		appStateChangeGenerator := hello.NewAppStateChangeGenerator()

		client := hello.NewPairingClient(
			c.String(peerNameFlag.Name),
			c.String(serverUrlFlag.Name),
			&wg.Config{
				PrivateKey: pkey.String(),
				Subnet:     "32",
			},

			hello.KeyPair{
				PublicKey:  pkey.PublicKey().String(),
				PrivateKey: pkey.String(),
			},
			wg.NewWatcher(c.String(wireguardConfigFilePathFlag.Name)),
			hello.NewJSONPairingEncoder(),
			hello.NewHTTPClientPairingTransport(c.String(serverUrlFlag.Name)),
		)
		var pairingResponse hello.PairingResponse
		for {
			var err error
			if pairingResponse, err = client.Pair(); err != nil {
				logrus.Error(err)
				time.Sleep(c.Duration(helloRetryIntervalFlag.Name))
				continue
			}
			break
		}

		logrus.Infof("Paired with server, assigned IP: %s", pairingResponse.AssignedIP)
		go localListenerRegistry.Watch(getStateManager(c).Changes(), make(chan bool))
		go remoteListenerRegistry.Watch(appStateChangeGenerator.Changes(), make(chan bool))

		sc, scErr := hello.NewHTTPSyncingClient(
			appStateChangeGenerator,
			hello.NewJSONSyncEncoder(),
			time.Second*5,
			hello.NewPeerEnrichingAppSource(
				c.String(peerNameFlag.Name),
				localListenerRegistry,
			),
			pairingResponse,
		)
		if scErr != nil {
			logrus.Fatalf("Failed to create syncing client: %v", scErr)
		}
		sc.Start()

		return nil
	},
}
