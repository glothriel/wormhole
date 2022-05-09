package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

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
