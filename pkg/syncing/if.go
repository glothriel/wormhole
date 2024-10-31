package syncing

import "github.com/glothriel/wormhole/pkg/apps"

// AppSource is an interface for listing apps
type AppSource interface {
	List() ([]apps.App, error)
}
