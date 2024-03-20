package cmd

import (
	"fmt"
	"strconv"

	"github.com/glothriel/wormhole/pkg/admin"
	"github.com/glothriel/wormhole/pkg/auth"
	"github.com/glothriel/wormhole/pkg/grtn"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/ports"
	"github.com/glothriel/wormhole/pkg/server"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
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
	},
	Action: func(c *cli.Context) error {
		startPrometheusServer(c)
		wsTransportFactory, wsTransportFactoryErr := peers.NewWebsocketTransportFactory(
			c.String("host"),
			strconv.Itoa(c.Int("port")),
			c.String("path"),
		)
		if wsTransportFactoryErr != nil {
			return wsTransportFactoryErr
		}

		consentGatherer := admin.NewConsentGatherer()

		peerFactory := peers.NewDefaultPeerFactory(
			"my-server",
			auth.NewRSAAuthorizedTransportFactory(
				wsTransportFactory,
				getAcceptor(c, consentGatherer),
			),
		)
		var portOpenerFactory server.PortOpenerFactory
		if c.Bool("kubernetes") {
			portOpenerFactory = server.NewK8sServicePortOpenerFactory(
				c.String("kubernetes-namespace"),
				server.CSVToMap(c.String("kubernetes-labels")),
				server.NewPerAppPortOpenerFactory(
					ports.RandomPortFromARangeAllocator{
						Min: c.Int("port-range-min"),
						Max: c.Int("port-range-max"),
					},
				),
			)
		} else {
			var allocator ports.Allocator
			if c.Bool("port-use-range") {
				allocator = ports.RandomPortFromARangeAllocator{
					Min: c.Int("port-range-min"),
					Max: c.Int("port-range-max"),
				}
			} else {
				allocator = ports.RandomPortAllocator{}
			}
			portOpenerFactory = server.NewPerAppPortOpenerFactory(
				allocator,
			)
		}
		appExposer := server.NewDefaultAppExposer(
			portOpenerFactory,
		)
		transportServer := server.NewServer(
			peerFactory,
			appExposer,
		)
		adminServer := admin.NewWormholeAdminServer(
			fmt.Sprintf(":%d", c.Int("admin-port")),
			server.NewServerAppsListAdapter(appExposer),
			consentGatherer,
		)
		grtn.Go(func() {
			if listenErr := adminServer.Listen(); listenErr != nil {
				logrus.Fatal(listenErr)
			}
		})
		return transportServer.Start()
	},
}

func getAcceptor(c *cli.Context, consentGatherer *admin.ConsentGatherer) auth.Acceptor {
	if c.String("acceptor") != "server" {
		return auth.DummyAcceptor{}
	}
	serverAcceptor := admin.NewServerAcceptor(consentGatherer)
	if c.String("acceptor-storage-file-path") == "" {
		return auth.NewInMemoryCachingAcceptor(serverAcceptor)
	}
	return auth.NewInFileCachingAcceptor(
		c.String("acceptor-storage-file-path"),
		serverAcceptor,
	)
}
