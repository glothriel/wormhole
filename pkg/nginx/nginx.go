package nginx

import (
	"github.com/glothriel/wormhole/pkg/peers"
)

// StreamServer is a struct that holds components of Nginx configuration related to
// "server" directive
type StreamServer struct {
	File       string
	ListenPort int
	ProxyPass  string

	App peers.App
}
