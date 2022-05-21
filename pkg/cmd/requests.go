package cmd

import (
	"fmt"

	"github.com/glothriel/wormhole/pkg/auth"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var requestsServerFlag = &cli.StringFlag{
	Name:  "server",
	Value: "http://localhost:8081",
}

var requestsCommand *cli.Command = &cli.Command{
	Name:  "requests",
	Flags: []cli.Flag{},
	Subcommands: []*cli.Command{
		{
			Name: "list",
			Flags: []cli.Flag{
				requestsServerFlag,
			},
			Subcommands: []*cli.Command{},
			Action: func(c *cli.Context) error {
				requests, listErr := auth.ListPairingRequests(c.String("server"))
				if listErr != nil {
					return fmt.Errorf("Failed to list pairing requests: %w", listErr)
				}
				if len(requests) == 0 {
					logrus.Info("No pairing requests are awaiting approval")
					return nil
				}
				fmt.Println("The following fingerprints are awaiting pairing request:")
				for _, fp := range requests {
					fmt.Printf("%s\n", fp)
				}
				return nil
			},
		},
		{
			Name: "accept",
			Flags: []cli.Flag{
				requestsServerFlag,
			},
			ArgsUsage:   "<fingerprint> - the fingerprint of certificate you'd like to accept",
			Subcommands: []*cli.Command{},
			Action: func(c *cli.Context) error {
				fpToBeAccepted := c.Args().First()
				if fpToBeAccepted == "" {
					return fmt.Errorf("First argument to this command must be a fingerprint")
				}
				if acceptErr := auth.AcceptRequest(c.String("server"), fpToBeAccepted); acceptErr != nil {
					return fmt.Errorf("Failed to accept pairing request: %w", acceptErr)
				}
				fmt.Println("OK")
				return nil
			},
		},
		{
			Name: "decline",
			Flags: []cli.Flag{
				requestsServerFlag,
			},
			ArgsUsage:   "<fingerprint> - the fingerprint of certificate you'd like to decline",
			Subcommands: []*cli.Command{},
			Action: func(c *cli.Context) error {
				fpToBeDeclined := c.Args().First()
				if fpToBeDeclined == "" {
					return fmt.Errorf("First argument to this command must be a fingerprint")
				}
				if declineErr := auth.DeclineRequest(c.String("server"), fpToBeDeclined); declineErr != nil {
					return fmt.Errorf("Failed to decline pairing request: %w", declineErr)
				}
				fmt.Println("OK")
				return nil
			},
		},
	},
}
