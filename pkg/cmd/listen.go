package cmd

import (
	"fmt"
	"net/http"

	"github.com/glothriel/wormhole/pkg/hello"
	"github.com/glothriel/wormhole/pkg/k8s"
	"github.com/glothriel/wormhole/pkg/listeners"
	"github.com/glothriel/wormhole/pkg/nginx"
	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
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

var listenCommand *cli.Command = &cli.Command{
	Name: "listen",
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
		wgAddressFlag,
		wgSubnetFlag,
		wgPortFlag,
	},
	Action: func(c *cli.Context) error {
		startPrometheusServer(c)

		pkey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}

		appsExposedHere := listeners.NewApps(nginx.NewNginxExposer(
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
		appsExposedFromRemote := listeners.NewApps(effectiveExposer)

		go appsExposedHere.Watch(getAppStateChangeGenerator(c).Changes(), make(chan bool))

		remoteNginxAdapter := hello.NewAppStateChangeGenerator()
		go appsExposedFromRemote.Watch(remoteNginxAdapter.Changes(), make(chan bool))

		wgConfig := &wg.Config{
			Address:    c.String(wgAddressFlag.Name),
			Subnet:     c.String(wgSubnetFlag.Name),
			ListenPort: c.Int(wgPortFlag.Name),
			PrivateKey: pkey.String(),
		}
		peers := hello.NewInMemoryPeerStorage()
		syncTransport := hello.NewHTTPServerSyncingTransport(&http.Server{
			Addr: fmt.Sprintf("%s:%d", c.String(wgAddressFlag.Name), c.Int(intServerListenPort.Name)),
		})
		ss := hello.NewSyncingServer(
			remoteNginxAdapter,
			hello.NewPeerEnrichingAppSource("server", appsExposedHere),
			hello.NewJSONSyncEncoder(),
			syncTransport,
			peers,
		)
		watcher := wg.NewWatcher(c.String(wireguardConfigFilePathFlag.Name))
		watcher.Update(*wgConfig)
		peerTransport := hello.NewHTTPServerPairingTransport(&http.Server{
			Addr: c.String(extServerListenAddress.Name),
		})
		if c.String(inviteTokenFlag.Name) != "" {
			peerTransport = hello.NewPSKPairingServerTransport(
				c.String(inviteTokenFlag.Name),
				peerTransport,
			)
		}
		ps := hello.NewPairingServer(
			"server",
			fmt.Sprintf("%s:%d", c.String(wgPublicHostFlag.Name), c.Int(wgPortFlag.Name)),
			wgConfig,
			hello.KeyPair{
				PublicKey:  pkey.PublicKey().String(),
				PrivateKey: pkey.String(),
			},
			watcher,
			hello.NewJSONPairingEncoder(),
			peerTransport,
			hello.NewIPPool(c.String(wgAddressFlag.Name)),
			peers,
			[]hello.MetadataEnricher{syncTransport},
		)
		go ss.Start()
		ps.Start()
		return nil
	},
}
