package syncing

import "github.com/glothriel/wormhole/pkg/peers"

// AppSource is an interface for listing apps
type AppSource interface {
	List() ([]peers.App, error)
}
