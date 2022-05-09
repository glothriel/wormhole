package cmd

import (
	"github.com/glothriel/wormhole/pkg/testutils"
	"github.com/urfave/cli/v2"
)

var testserverCommand *cli.Command = &cli.Command{
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
}
