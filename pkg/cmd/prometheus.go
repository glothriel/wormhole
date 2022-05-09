package cmd

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func startPrometheusServer(c *cli.Context) {
	if !c.Bool("metrics") {
		return
	}
	metricsAddr := fmt.Sprintf("%s:%d", c.String("metrics-host"), c.Int("metrics-port"))
	http.Handle("/metrics", promhttp.Handler())
	logrus.Infof("Starting prometheus metrics server on %s", metricsAddr)
	go func() {
		if listenErr := http.ListenAndServe(metricsAddr, nil); listenErr != nil {
			logrus.Fatalf("Failed to start prometheus metrics server: %v", listenErr)
		}
	}()
}
