package nginx

import (
	"fmt"
	"path"
	"sync"

	"github.com/glothriel/wormhole/pkg/k8s/svcdetector"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

type StreamServer struct {
	File       string
	ListenPort int
	ProxyPass  string

	App peers.App
}

type ConfigGuard struct {
	prefix string
	path   string
	fs     afero.Fs

	reloader      Reloader
	portAllocator PortAllocator

	Servers []StreamServer
	lock    sync.Mutex
}

func (g *ConfigGuard) Watch(c chan svcdetector.AppStateChange, done chan bool) {
	for {
		func() {
			select {
			case appStageChange := <-c:
				g.lock.Lock()
				defer g.lock.Unlock()
				if appStageChange.State == svcdetector.AppStateChangeAdded {
					port, portErr := g.portAllocator.Allocate()
					if portErr != nil {
						logrus.Errorf("Could not allocate port: %v", portErr)
						return
					}
					server := StreamServer{
						ListenPort: port,
						ProxyPass:  appStageChange.App.Address,
						File: fmt.Sprintf(
							"%s-%s.conf", g.prefix, appStageChange.App.Name,
						),
						App: appStageChange.App,
					}
					g.Servers = append(g.Servers, server)
					afero.WriteFile(g.fs, path.Join(g.path, server.File), []byte(fmt.Sprintf(`
# [%s] %s
server {
	listen %d;
	proxy_pass %s;
}
`,
						server.App.Peer,
						server.App.Name,
						server.ListenPort,
						server.ProxyPass,
					)), 0644)
				} else if appStageChange.State == svcdetector.AppStateChangeWithdrawn {
					g.fs.Remove(path.Join(g.path, fmt.Sprintf(
						"%s-%s.conf", g.prefix, appStageChange.App.Name,
					)))
					for i, server := range g.Servers {
						if server.ProxyPass == appStageChange.App.Address {
							g.portAllocator.Return(server.ListenPort)
							g.Servers = append(g.Servers[:i], g.Servers[i+1:]...)
							break
						}
					}
				}
				if reloaderErr := g.reloader.Reload(); reloaderErr != nil {
					logrus.Errorf("Could not reload nginx: %v", reloaderErr)
				}
			case <-done:
				return
			}
		}()
	}
}

func NewNginxConfigGuard(path, confPrefix string, reloader Reloader) *ConfigGuard {
	return &ConfigGuard{
		path:   path,
		prefix: confPrefix,
		fs:     afero.NewOsFs(),

		reloader:      reloader,
		portAllocator: NewRangePortAllocator(20000, 25000),
	}
}
