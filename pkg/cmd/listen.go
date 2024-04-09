package cmd

import (
	"fmt"

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
		Name:  "wg-host",
		Value: "10.188.0.1",
	}

	wgSubnetFlag *cli.StringFlag = &cli.StringFlag{
		Name:  "wg-subnet-mask",
		Value: "24",
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
		stateManagerPathFlag,
		nginxExposerConfdPathFlag,
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

		go localListenerRegistry.Watch(getStateManager(c).Changes(), make(chan bool))

		remoteNginxAdapter := hello.NewAppStateChangeGenerator()
		go remoteListenerRegistry.Watch(remoteNginxAdapter.Changes(), make(chan bool))

		return hello.NewServer(
			fmt.Sprintf("%s:%d", c.String(wgAddressFlag.Name), c.Int(intServerListenPort.Name)),
			c.String(extServerListenAddress.Name),
			pkey.PublicKey().String(),
			"wormhole-server-chart.server.svc.cluster.local:51820",
			&wg.Config{
				Address:    c.String(wgAddressFlag.Name),
				Subnet:     c.String(wgSubnetFlag.Name),
				ListenPort: c.Int(wgPortFlag.Name),
				PrivateKey: pkey.String(),
			},
			localListenerRegistry,
			remoteNginxAdapter,
			wg.NewWriter(c.String(wireguardConfigFilePathFlag.Name)),
		).Listen()
	},
}
