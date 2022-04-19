package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/glothriel/wormhole/pkg/admin"
	"github.com/glothriel/wormhole/pkg/auth"
	"github.com/glothriel/wormhole/pkg/client"
	"github.com/glothriel/wormhole/pkg/k8s/svcdetector"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/glothriel/wormhole/pkg/ports"
	"github.com/glothriel/wormhole/pkg/server"
	"github.com/glothriel/wormhole/pkg/testutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func getExposedApps(c *cli.Context) []peers.App {
	upstreams := []peers.App{}
	for _, upstreamDefinition := range c.StringSlice("expose") {
		splitDefinition := strings.Split(upstreamDefinition, ",")
		if len(splitDefinition) == 1 && len(strings.Split(upstreamDefinition, "=")) == 1 {
			upstreams = append(upstreams, peers.App{
				Name:    splitDefinition[0],
				Address: splitDefinition[0],
			})
			continue
		}
		var name, address string
		for _, wholeDef := range splitDefinition {
			fields := strings.Split(wholeDef, "=")
			if len(fields) != 2 {
				logrus.Fatalf("Invalid expose value %s: shold consist of comma-separated key=value pairs", wholeDef)
			}
			if fields[0] == "name" {
				name = fields[1]
			} else if fields[0] == "address" {
				address = fields[1]
			} else {
				logrus.Fatalf("Invalid expose value %s: could not recongnize `%s` field", wholeDef, fields[0])
			}
		}
		if name == "" || address == "" {
			logrus.Fatalf("You need to set both `name` and `address` fields, got: %s", upstreamDefinition)
		}
		upstreams = append(upstreams, peers.App{
			Name:    name,
			Address: address,
		})
	}
	if len(upstreams) < 1 {
		logrus.Fatal(
			"You need to provide at least one app, that will be exposed on this host to join the mesh",
		)
	}
	return upstreams
}

func getAppStateManager(c *cli.Context) client.AppStateManager {
	if c.Bool("kubernetes") {
		config, inClusterConfigErr := rest.InClusterConfig()
		if inClusterConfigErr != nil {
			logrus.Fatal(inClusterConfigErr)
		}
		clientset, clientSetErr := kubernetes.NewForConfig(config)
		if clientSetErr != nil {
			logrus.Fatal(clientSetErr)
		}
		servicesClient := clientset.CoreV1().Services("")
		return svcdetector.NewK8sAppStateManager(
			svcdetector.NewDefaultServiceRepository(servicesClient),
			time.Second*30,
		)
	}
	return client.NewStaticAppStateManager(getExposedApps(c))
}

//nolint:funlen
func main() {
	app := &cli.App{
		Name:                 "wormhole",
		Usage:                "Wormhole is an utility to create reverse websocket tunnels, similar to ngrok",
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			{
				Name:  "mesh",
				Usage: "Allows listening and joining wormhole mesh",
				Subcommands: []*cli.Command{
					{
						Name: "listen",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "host",
								Value: "0.0.0.0",
							},
							&cli.IntFlag{
								Name:  "port",
								Value: 8080,
							},
							&cli.StringFlag{
								Name:  "labels",
								Value: "application=wormhole-server",
							},
							&cli.BoolFlag{
								Name: "kubernetes",
							},
							&cli.StringFlag{
								Name:  "kubernetes-namespace",
								Value: "wormhole",
							},
							&cli.IntFlag{
								Name:  "kubernetes-pod-port-range-min",
								Value: 30000,
							},
							&cli.IntFlag{
								Name:  "kubernetes-pod-port-range-max",
								Value: 30499,
							},
						},
						Action: func(c *cli.Context) error {
							wsTransportFactory, wsTransportFactoryErr := peers.NewWebsocketTransportFactory(
								c.String("host"),
								strconv.Itoa(c.Int("port")),
							)
							if wsTransportFactoryErr != nil {
								return wsTransportFactoryErr
							}

							peerFactory := peers.NewDefaultPeerFactory(
								"my-server",
								auth.NewRSAAuthorizedTransportFactory(wsTransportFactory),
							)
							var portOpenerFactory server.PortOpenerFactory
							if c.Bool("kubernetes") {
								portOpenerFactory = server.NewK8sServicePortOpenerFactory(
									c.String("kubernetes-namespace"),
									server.CSVToMap(c.String("labels")),
									server.NewPerAppPortOpenerFactory(
										ports.RandomPortFromARangeAllocator{
											Min: c.Int("kubernetes-pod-port-range-min"),
											Max: c.Int("kubernetes-pod-port-range-max"),
										},
									),
								)
							} else {
								portOpenerFactory = server.NewPerAppPortOpenerFactory(
									ports.RandomPortAllocator{},
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
								":8081",
								server.NewServerAppsListAdapter(appExposer),
							)
							go func() {
								if listenErr := adminServer.Listen(); listenErr != nil {
									logrus.Fatal(listenErr)
								}
							}()
							return transportServer.Start()
						},
					},
					{
						Name: "join",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "server",
								Value: "ws://127.0.0.1:8080",
							},
							&cli.StringSliceFlag{
								Name: "expose",
							},
							&cli.BoolFlag{
								Name: "kubernetes",
							},
							&cli.StringFlag{
								Name:  "name",
								Value: "default",
							},
						},
						Action: func(c *cli.Context) error {
							transport, transportErr := peers.NewWebsocketClientTransport(c.String("server"))
							if transportErr != nil {
								return transportErr
							}
							keyPairProvider, keyPairProviderErr := auth.NewStoredInFilesKeypairProvider("/tmp")
							if keyPairProviderErr != nil {
								return fmt.Errorf("Failed to initialize key pair provider: %w", keyPairProviderErr)
							}
							rsaTransport, rsaTransportErr := auth.NewRSAAuthorizedTransport(transport, keyPairProvider)
							if rsaTransportErr != nil {
								return rsaTransportErr
							}
							peer, peerErr := peers.NewDefaultPeer(
								c.String("name"),
								rsaTransport,
							)
							if peerErr != nil {
								return peerErr
							}
							return client.NewExposer(peer).Expose(getAppStateManager(c))
						},
					},
				},
			},
			{
				Name:  "testserver",
				Usage: "",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "port",
						Value: 1234,
					},
					&cli.StringFlag{
						Name:  "response",
						Value: "Hello world!",
					},
				},
				Action: func(c *cli.Context) error {
					return testutils.RunTestServer(c.Int("port"), c.String("response"))
				},
			},
		},
		Version: "0.0.1",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Be more verbose when logging stuff",
			}, &cli.BoolFlag{
				Name:  "trace",
				Usage: "Be even more verbose when logging stuff",
			},
		},

		Before: setLogLevel,
		ExitErrHandler: func(context *cli.Context, theErr error) {
			if logrus.GetLevel() != logrus.DebugLevel {
				logrus.Error(
					"Wormhole command failed. For verbose output, please use `wormhole --debug <your-command>`",
				)
			}
		},
	}

	if runErr := app.Run(os.Args); runErr != nil {
		log.Fatal(runErr)
	}
}

func setLogLevel(c *cli.Context) error {
	if c.IsSet("trace") {
		logrus.Warn("Log level set to trace")
		logrus.SetLevel(logrus.TraceLevel)
	} else if c.IsSet("debug") {
		logrus.Warn("Log level set to debug")
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.Info("Log level set to info")
		logrus.SetLevel(logrus.InfoLevel)
	}
	return nil
}
