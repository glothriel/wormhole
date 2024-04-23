package nginx

import (
	"github.com/glothriel/wormhole/pkg/peers"
)

type StreamServer struct {
	File       string
	ListenPort int
	ProxyPass  string

	App peers.App
}
