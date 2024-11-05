// Package listeners exposes
package listeners

import (
	"github.com/glothriel/wormhole/pkg/apps"
	"github.com/glothriel/wormhole/pkg/k8s/svcdetector"
	"github.com/sirupsen/logrus"
)

// Exposer reacts to changes in the app state and perform necessary actions like opening sockets,
// creating kube services, etc.
type Exposer interface {
	Add(app apps.App) (apps.App, error)
	Withdraw(app apps.App) error
	WithdrawAll() error
}

type noOpExposer struct {
}

func (e *noOpExposer) Add(app apps.App) (apps.App, error) {
	return app, nil
}

func (e *noOpExposer) Withdraw(_ apps.App) error {
	return nil
}

func (e *noOpExposer) WithdrawAll() error {
	return nil
}

// NewNoOpExposer creates a new no-op exposer
func NewNoOpExposer() Exposer {
	return &noOpExposer{}
}

// Registry is a registry of apps, that also listens for changes in the app state and triggers the exposer
type Registry struct {
	Exposer Exposer
	apps    []apps.App
}

// Watch listens for changes in the app state and triggers the exposer
func (g *Registry) Watch(c chan svcdetector.AppStateChange, done chan bool) { // nolint: gocognit
	for {
		select {
		case appStageChange := <-c:
			func() {
				if appStageChange.State == svcdetector.AppStateChangeAdded {
					logrus.Infof("App local.%s added", appStageChange.App.Name)
					newApp, createErr := g.Exposer.Add(appStageChange.App)
					if createErr != nil {
						logrus.Errorf("Could not create listener: %v", createErr)
						return
					}
					g.apps = append(g.apps, newApp)
				} else if appStageChange.State == svcdetector.AppStateChangeWithdrawn {
					logrus.Infof("App local.%s withdrawn", appStageChange.App.Name)
					if withdrawErr := g.Exposer.Withdraw(appStageChange.App); withdrawErr != nil {
						logrus.Errorf("Could not withdraw app: %v", withdrawErr)
					}
					for i, app := range g.apps {
						if app.Name == appStageChange.App.Name && appStageChange.App.Peer == app.Peer {
							g.apps = append(g.apps[:i], g.apps[i+1:]...)
							break
						}
					}
				}
			}()
		case <-done:
			return
		}
	}
}

// List returns the list of apps
func (g *Registry) List() ([]apps.App, error) {
	return g.apps, nil
}

// NewApps creates a new registry of apps
func NewApps(r Exposer) *Registry {
	return &Registry{
		Exposer: r,
	}
}
