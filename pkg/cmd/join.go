package cmd

import (
	"time"

	"github.com/glothriel/wormhole/pkg/hello"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var joinCommand *cli.Command = &cli.Command{
	Name: "join",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "server",
			Value: "ws://127.0.0.1:8080/wh/tunnel",
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
		&cli.StringFlag{
			Name:  "keypair-storage-path",
			Value: "/tmp",
		},
	},
	Action: func(c *cli.Context) error {
		startPrometheusServer(c)
		helloClient := hello.NewClient(c.String("server"), "dev1")
		for {
			if _, err := helloClient.Hello(); err != nil {
				logrus.Error(err)
				time.Sleep(time.Second * 5)
				continue
			}
			break
		}
		time.Sleep(time.Hour * 24)
		return nil
	},
}
