package cmd

import (
	"fmt"
	"net/http"
	"time"

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
		server := &http.Server{
			Addr:              metricsAddr,
			ReadHeaderTimeout: 3 * time.Second,
		}

		if listenErr := server.ListenAndServe(); listenErr != nil {
			logrus.Fatalf("Failed to start prometheus metrics server: %v", listenErr)
		}
	}()
}
