package main

import (
	"github.com/glothriel/wormhole/pkg/cmd"
	"github.com/sirupsen/logrus"
)

//nolint:funlen
func main() {
	logrus.Error("Starting wormhole...")
	cmd.Run()
}
