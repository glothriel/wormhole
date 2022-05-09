package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/glothriel/wormhole/pkg/auth"
	"github.com/glothriel/wormhole/pkg/client"
	"github.com/glothriel/wormhole/pkg/k8s/svcdetector"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var joinCommand *cli.Command = &cli.Command{
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
		startPrometheusServer(c)
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
				logrus.Fatalf("Invalid expose value %s: should consist of comma-separated key=value pairs", wholeDef)
			}
			if fields[0] == "name" {
				name = fields[1]
			} else if fields[0] == "address" {
				address = fields[1]
			} else {
				logrus.Fatalf("Invalid expose value %s: could not recognize `%s` field", wholeDef, fields[0])
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
