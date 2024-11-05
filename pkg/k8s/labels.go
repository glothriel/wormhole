package k8s

import (
	"strings"

	"github.com/glothriel/wormhole/pkg/apps"
)

// CSVToMap converts key1=v1,key2=v2 entries into flat string map
func CSVToMap(csv string) map[string]string {
	theMap := map[string]string{}
	if csv == "" {
		return theMap
	}
	for _, kvPair := range strings.Split(csv, ",") {
		parsedKVPair := strings.Split(kvPair, "=")
		theMap[parsedKVPair[0]] = strings.Join(parsedKVPair[1:], "=")
	}
	return theMap
}

const exposedByLabel = "wormhole.glothriel.github.com/exposed-by"
const exposedAppLabel = "wormhole.glothriel.github.com/exposed-app"
const exposedPeerLabel = "wormhole.glothriel.github.com/exposed-peer"

func resourceLabels(app apps.App) map[string]string {
	return map[string]string{
		exposedByLabel:   "wormhole",
		exposedAppLabel:  app.Name,
		exposedPeerLabel: app.Peer,
	}
}
