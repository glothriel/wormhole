package cmd

import (
	"github.com/glothriel/wormhole/pkg/hello"
	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/urfave/cli/v2"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var listenCommand *cli.Command = &cli.Command{
	Name: "listen",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "host",
			Value: "0.0.0.0",
			Usage: "Host the tunnel server will be listening on",
		},
		&cli.IntFlag{
			Name:  "port",
			Value: 8080,
			Usage: "Port the tunnel server will be listening on",
		},
		&cli.IntFlag{
			Name:  "admin-port",
			Value: 8081,
			Usage: "Port the admin server will be listening on",
		},
		&cli.StringFlag{
			Name:  "path",
			Value: "/wh/tunnel",
			Usage: "Path under which the tunnel server will expose the tunnel entrypoint. All other paths will be 404",
		},
		&cli.BoolFlag{
			Name:  "port-use-range",
			Value: false,
			Usage: "Uses fixed port range for allocations",
		},
		&cli.IntFlag{
			Name:  "port-range-min",
			Value: 30000,
			Usage: "Port range for allocations of new proxy services",
		},
		&cli.IntFlag{
			Name:  "port-range-max",
			Value: 30499,
			Usage: "Port range for allocations of new proxy services",
		},
		&cli.BoolFlag{
			Name:  "kubernetes",
			Usage: "Enables kubernetes integration",
		},
		&cli.StringFlag{
			Name:  "kubernetes-namespace",
			Value: "wormhole",
			Usage: "Namespace to create the proxy services in",
		},
		&cli.StringFlag{
			Name:  "kubernetes-labels",
			Value: "application=wormhole-server",
			Usage: "Labels that will be set on proxy service, must match the labels of wormhole server pod",
		},
		&cli.StringFlag{
			Name:  "acceptor",
			Value: "server",
			Usage: "How would you like to accept pairing requests? `server` waits for approval, every " +
				"other value triggers DummyAcceptor, that automatically blindly accepts all pairing requests",
		},
		&cli.StringFlag{
			Name:  "acceptor-storage-file-path",
			Value: "",
			Usage: "A file, that holds information about previously accepted fingerprints. If left entry, " +
				"the information will be stored in memory",
		},
		&cli.StringFlag{
			Name:  "wg-address",
			Value: "10.188.0.1",
		},
		&cli.StringFlag{
			Name:  "wg-subnet",
			Value: "24",
		},
		&cli.StringFlag{
			Name:  "wg-privkey",
			Value: "",
		},
		&cli.IntFlag{
			Name:  "wg-port",
			Value: 51820,
		},
	},
	Action: func(c *cli.Context) error {
		startPrometheusServer(c)

		pkey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}
		hello.NewServer(
			"0.0.0.0:8081",
			pkey.PublicKey().String(),
			"wormhole-server-chart.server.svc.cluster.local:51820",
			&wg.Cfg{
				Address:    c.String("wg-address"),
				Subnet:     c.String("wg-subnet"),
				ListenPort: c.Int("wg-port"),
				PrivateKey: pkey.String(),
			},
		).Listen()
		return nil
	},
}
