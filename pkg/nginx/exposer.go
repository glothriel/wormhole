package nginx

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/glothriel/wormhole/pkg/listeners"
	"github.com/glothriel/wormhole/pkg/peers"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

type NginxExposer struct {
	prefix string
	path   string
	fs     afero.Fs

	reloader Reloader
	ports    PortAllocator
}

func (n *NginxExposer) Add(app peers.App) (peers.App, error) {
	port, portErr := n.ports.Allocate()
	if portErr != nil {
		return peers.App{}, fmt.Errorf("Could not allocate port: %v", portErr)
	}
	server := StreamServer{
		ListenPort: port,
		ProxyPass:  app.Address,
		File: fmt.Sprintf(
			"%s-%s-%s.conf", n.prefix, app.Peer, app.Name,
		),
		App: app,
	}

	if writeErr := afero.WriteFile(n.fs, path.Join(n.path, server.File), []byte(fmt.Sprintf(`
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

	if reloaderErr := n.reloader.Reload(); reloaderErr != nil {
		logrus.Errorf("Could not reload NGINX: %v", reloaderErr)
	}
	return peers.WithAddress(app, fmt.Sprintf("localhost:%d", port)), nil
}

func (n *NginxExposer) Withdraw(app peers.App) error {
	removeErr := n.fs.Remove(path.Join(n.path, fmt.Sprintf(
		"%s-%s-%s.conf", n.prefix, app.Peer, app.Name,
	)))

	if reloaderErr := n.reloader.Reload(); reloaderErr != nil {
		logrus.Errorf("Could not reload NGINX: %v", reloaderErr)
	}
	return removeErr
}

func (n *NginxExposer) WithdrawAll() error {
	filesToClean := make([]string, 0)
	if walkErr := afero.Walk(n.fs, n.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasPrefix(info.Name(), n.prefix) || !strings.HasSuffix(info.Name(), ".conf") {
			return nil
		}
		filesToClean = append(filesToClean, path)
		return nil
	}); walkErr != nil {
		return fmt.Errorf("Could not walk through directory: %v", walkErr)
	}
	deleted := 0
	for _, file := range filesToClean {
		removeErr := n.fs.Remove(file)
		if removeErr != nil {
			logrus.Errorf("Could not remove file %s: %v", file, removeErr)
		} else {
			deleted++
			logrus.Infof("Cleaned up NGINX config file upon startup %s", file)
		}
	}
	if deleted > 0 {
		if reloaderErr := n.reloader.Reload(); reloaderErr != nil {
			logrus.Errorf("Could not reload NGINX: %v", reloaderErr)
		}
	}

	return nil
}

func NewNginxExposer(path, confPrefix string, reloader Reloader, allocator PortAllocator) listeners.Exposer {
	fs := afero.NewOsFs()
	cg := &NginxExposer{
		path:   path,
		prefix: confPrefix,
		fs:     fs,

		reloader: reloader,
		ports:    allocator,
	}
	createErr := fs.MkdirAll(path, 0755)
	if createErr != nil && createErr != afero.ErrDestinationExists {
		logrus.Fatalf("Could not create NGINX config directory at %s: %v", path, createErr)
	}

	if cleanErr := cg.WithdrawAll(); cleanErr != nil {
		logrus.Errorf("Could not clean NGINX config directory: %v", cleanErr)
	}

	return cg
}
