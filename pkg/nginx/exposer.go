// Package nginx implements wormhole integration with NGINX as a proxy server.
package nginx

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/glothriel/wormhole/pkg/apps"
	"github.com/glothriel/wormhole/pkg/listeners"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

// Exposer is an Exposer implementation that uses NGINX as a proxy server
type Exposer struct {
	prefix   string
	path     string
	fs       afero.Fs
	listener Listener

	reloader Reloader
	ports    PortAllocator
}

// Add implements listeners.Exposer
func (n *Exposer) Add(app apps.App) (apps.App, error) {
	port, portErr := n.ports.Allocate()
	if portErr != nil {
		return apps.App{}, fmt.Errorf("Could not allocate port: %v", portErr)
	}
	server := StreamServer{
		ProxyPass: app.Address,
		File:      nginxConfigPath(n.prefix, app),
		App:       app,
	}
	listenBlock := ""
	listenAddrs, addrsErr := n.listener.Addrs(port)
	if addrsErr != nil {
		return apps.App{}, fmt.Errorf("Could not get listener addresses: %v", addrsErr)
	}
	for _, addr := range listenAddrs {
		listenBlock += fmt.Sprintf("	listen %s;\n", addr)
	}
	if writeErr := afero.WriteFile(n.fs, path.Join(n.path, server.File), []byte(fmt.Sprintf(`
# [%s] %s
server {
%s
	proxy_pass %s;
}
`,
		server.App.Peer,
		server.App.Name,
		listenBlock,
		server.ProxyPass,
	)), 0644); writeErr != nil {
		logrus.Errorf("Could not write NGINX config file: %v", writeErr)
	} else {
		logrus.Infof("Created NGINX config file %s", server.File)
	}

	if reloaderErr := n.reloader.Reload(); reloaderErr != nil {
		logrus.Errorf("Could not reload NGINX: %v", reloaderErr)
	}
	return apps.WithAddress(app, fmt.Sprintf("localhost:%d", port)), nil
}

// Withdraw implements listeners.Exposer
func (n *Exposer) Withdraw(app apps.App) error {
	path := path.Join(n.path, nginxConfigPath(n.prefix, app))
	removeErr := n.fs.Remove(path)

	if removeErr != nil {
		if os.IsNotExist(removeErr) {
			logrus.Debugf("Expected NGINX config file `%s` does not exist: %v.", path, removeErr)
		} else {
			return fmt.Errorf("Could not remove NGINX config file: %v", removeErr)
		}
	} else {
		logrus.Infof("Removed NGINX config file %s", path)
	}
	if reloaderErr := n.reloader.Reload(); reloaderErr != nil {
		logrus.Errorf("Could not reload NGINX: %v", reloaderErr)
	}
	return nil
}

// WithdrawAll implements listeners.Exposer
func (n *Exposer) WithdrawAll() error {
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

// NewNginxExposer creates a new NGINX exposer
func NewNginxExposer(
	path, confPrefix string, reloader Reloader, allocator PortAllocator, listener Listener,
) listeners.Exposer {
	fs := afero.NewOsFs()
	cg := &Exposer{
		path:   path,
		prefix: confPrefix,
		fs:     fs,

		reloader: reloader,
		ports:    allocator,
		listener: listener,
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

func nginxConfigPath(prefix string, app apps.App) string {
	if app.Peer == "" {
		return fmt.Sprintf("%s-%s.conf", prefix, app.Name)
	}
	return fmt.Sprintf("%s-%s-%s.conf", prefix, app.Peer, app.Name)
}
