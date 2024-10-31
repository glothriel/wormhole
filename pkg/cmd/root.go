package cmd

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

var debugFlag = &cli.BoolFlag{
	Name:  "debug",
	Usage: "Be more verbose when logging stuff",
}

// Run starts wormgole
func Run() {
	app := &cli.App{
		Name: "wormhole",
		Usage: ("Wormhole is an utility to create reverse websocket tunnels, " +
			"similar to ngrok, but designed to be used in a kubernetes cluster"),
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			serverCommand,
			clientCommand,
			testserverCommand,
		},
		Version: projectVersion,
		Flags: []cli.Flag{
			debugFlag,
			&cli.BoolFlag{
				Name:  "trace",
				Usage: "Be even more verbose when logging stuff",
			},
			&cli.BoolFlag{
				Name:  "metrics",
				Usage: "Start prometheus metrics server",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "metrics-host",
				Value: "0.0.0.0",
			},
			&cli.IntFlag{
				Name:  "metrics-port",
				Value: 8090,
			},
		},

		Before: setLogLevel,
	}

	if runErr := app.Run(os.Args); runErr != nil {
		log.Fatal(runErr)
	}
}
