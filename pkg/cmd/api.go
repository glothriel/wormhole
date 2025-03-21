package cmd

import (
	"github.com/glothriel/wormhole/pkg/api"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func configureAPIServer(cliCtx *cli.Context) api.ServerSettings {
	username := cliCtx.String(basicAuthUsernameFlag.Name)
	password := cliCtx.String(basicAuthPasswordFlag.Name)
	settings := api.NewServerSettings().WithDebug(cliCtx.Bool("debug"))
	if username != "" && password != "" {
		settings = settings.WithBasicAuth(username, password)
	} else {
		logrus.Info(
			"State-changing API endpoints will not be enabled - " +
				"either basic auth username or password is missing",
		)
	}
	return settings
}
