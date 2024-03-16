package cmd

import (
	"log"
	"os"

	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
	"github.com/urfave/cli/v2"
)

// Run starts wormgole
func Run() {
	app := &cli.App{
		Name:                 "wormhole",
		Usage:                "Wormhole is an utility to create reverse websocket tunnels, similar to ngrok",
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			listenCommand,
			joinCommand,
			requestsCommand,
			testserverCommand,
		},
		Version: projectVersion,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Be more verbose when logging stuff",
			},
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
			&cli.StringFlag{
				Name:  "jaeger-service-name",
				Value: "default",
			},
			&cli.StringFlag{
				Name:  "jaeger-endpoint",
				Value: "default",
			},
			&cli.IntFlag{
				Name:  "metrics-port",
				Value: 8090,
			},
		},
		Action: func(ctx *cli.Context) error {
			cfg := config.Configuration{
				ServiceName: ctx.String("jaeger-service-name"),
				Sampler: &config.SamplerConfig{
					Type:  jaeger.SamplerTypeConst,
					Param: 1,
				},
				Reporter: &config.ReporterConfig{
					LogSpans: true,
				},
			}

			tracer, _, err := cfg.NewTracer()
			if err != nil {
				panic(err)
			}
			opentracing.SetGlobalTracer(tracer)
			return nil
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
