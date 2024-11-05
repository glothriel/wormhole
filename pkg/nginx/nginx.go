package nginx

import "github.com/glothriel/wormhole/pkg/apps"

// StreamServer is a struct that holds components of Nginx configuration related to
// "server" directive
type StreamServer struct {
	File      string
	ProxyPass string

	App apps.App
}
