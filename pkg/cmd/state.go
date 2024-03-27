package cmd

import (
	"time"

	"github.com/glothriel/wormhole/pkg/k8s/svcdetector"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func getStateManager(peerDetector svcdetector.PeerDetector) svcdetector.AppStateManager {
	config, inClusterConfigErr := rest.InClusterConfig()
	if inClusterConfigErr != nil {
		logrus.Fatal(inClusterConfigErr)
	}
	dynamicClient, clientSetErr := dynamic.NewForConfig(config)
	if clientSetErr != nil {
		logrus.Fatal(clientSetErr)
	}
	return svcdetector.NewK8sAppStateManager(
		svcdetector.NewDefaultServiceRepository(dynamicClient, peerDetector),
		time.Second*30,
	)
}
