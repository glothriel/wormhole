package listeners

import (
	"github.com/glothriel/wormhole/pkg/k8s/svcdetector"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
)

type Exposer interface {
	Add(app peers.App) (peers.App, error)
	Withdraw(app peers.App) error
	WithdrawAll() error
}

type noOpExposer struct {
}

func (e *noOpExposer) Add(app peers.App) (peers.App, error) {
	return app, nil
}

func (e *noOpExposer) Withdraw(app peers.App) error {
	return nil
}

func (e *noOpExposer) WithdrawAll() error {
	return nil
}

func NewNoOpExposer() Exposer {
	return &noOpExposer{}
}

type Registry struct {
	Exposer Exposer
	apps    []peers.App
}

func (g *Registry) Watch(c chan svcdetector.AppStateChange, done chan bool) {
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

func (g *Registry) List() ([]peers.App, error) {
	return g.apps, nil
}

func NewApps(r Exposer) *Registry {
	return &Registry{
		Exposer: r,
	}
}
