package cmd

import (
	"time"

	"github.com/glothriel/wormhole/pkg/k8s/svcdetector"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func getStateManager(c *cli.Context) svcdetector.AppStateManager {
	if c.Bool(kubernetesFlag.Name) {
		config, inClusterConfigErr := rest.InClusterConfig()
		if inClusterConfigErr != nil {
			logrus.Panic(inClusterConfigErr)
		}
		dynamicClient, clientSetErr := dynamic.NewForConfig(config)
		if clientSetErr != nil {
			logrus.Panic(clientSetErr)
		}
		return svcdetector.NewK8sAppStateManager(
			svcdetector.NewDefaultServiceRepository(dynamicClient),
			time.Second*30,
		)
	} else if c.String(stateManagerPathFlag.Name) != "" {
		return svcdetector.NewDirectoryMonitoringAppStateManager(
			c.String(stateManagerPathFlag.Name),
			afero.NewOsFs(),
		)
	} else {
		logrus.Fatalf("No state manager specified, use --%s or --%s", kubernetesFlag.Name, stateManagerPathFlag.Name)
		return nil
	}
}
