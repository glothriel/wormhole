package nginx

import (
	"fmt"
	"os"
	"path"
	"strings"

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

type ConfdGuard struct {
	prefix string
	path   string
	fs     afero.Fs

	reloader      Reloader
	portAllocator PortAllocator

	Servers []StreamServer
}

func (g *ConfdGuard) RemoveAll() error {
	filesToClean := make([]string, 0)
	if walkErr := afero.Walk(g.fs, g.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasPrefix(info.Name(), g.prefix) || !strings.HasSuffix(info.Name(), ".conf") {
			return nil
		}
		filesToClean = append(filesToClean, path)
		return nil
	}); walkErr != nil {
		return fmt.Errorf("Could not walk through directory: %v", walkErr)
	}
	for _, file := range filesToClean {
		removeErr := g.fs.Remove(file)
		if removeErr != nil {
			logrus.Errorf("Could not remove file %s: %v", file, removeErr)
		} else {
			logrus.Infof("Cleaned up NGINX config file upon startup %s", file)
		}
	}
	return nil
}

func (g *ConfdGuard) Watch(c chan svcdetector.AppStateChange, done chan bool) {
	for {
		select {
		case appStageChange := <-c:
			func() {
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
							"%s-%s-%s.conf", g.prefix, appStageChange.App.Peer, appStageChange.App.Name,
						),
						App: appStageChange.App,
					}
					g.Servers = append(g.Servers, server)
					if writeErr := afero.WriteFile(g.fs, path.Join(g.path, server.File), []byte(fmt.Sprintf(`
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
					)), 0644); writeErr != nil {
						logrus.Errorf("Could not write NGINX config file: %v", writeErr)

					} else {
						logrus.Infof("Created NGINX config file %s", server.File)
					}
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
					logrus.Errorf("Could not reload NGINX: %v", reloaderErr)
				}
			}()
		case <-done:
			return
		}

	}
}

func NewConfdGuard(path, confPrefix string, reloader Reloader, allocator PortAllocator) *ConfdGuard {
	cg := &ConfdGuard{
		path:   path,
		prefix: confPrefix,
		fs:     afero.NewOsFs(),

		reloader:      reloader,
		portAllocator: allocator,
	}
	if cleanErr := cg.RemoveAll(); cleanErr != nil {
		logrus.Errorf("Could not clean NGINX config directory: %v", cleanErr)
	}

	return cg
}
