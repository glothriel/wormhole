package wg

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"text/template"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

type Peer struct {
	PublicKey  string
	AllowedIPs string
	Endpoint   string

	PersistentKeepalive int
}

type Config struct {
	Address    string
	Subnet     string
	ListenPort int
	PrivateKey string

	EnableKeepAlive bool

	Peers []Peer
}

func (c *Config) Upsert(p Peer) {
	// Replace if AllowedIPs is the same
	for i, peer := range c.Peers {
		if peer.AllowedIPs == p.AllowedIPs {
			logrus.Warnf("Peer with AllowedIPs %s already exists, replacing with new one", p.AllowedIPs)
			c.Peers[i] = p
			return
		}
	}

	c.Peers = append(c.Peers, p)
}

func (c *Config) DeleteByPublicKey(publicKey string) {
	for i, peer := range c.Peers {
		if peer.PublicKey == publicKey {
			c.Peers = append(c.Peers[:i], c.Peers[i+1:]...)
			return
		}
	}
}

var theTemplate string = `[Interface]
Address = {{.Address}}/{{.Subnet}}
{{if .ListenPort}}ListenPort = {{.ListenPort}}{{end}}
PrivateKey = {{.PrivateKey}}

{{range .Peers}}
[Peer]
PublicKey = {{ .PublicKey }}
AllowedIPs = {{ .AllowedIPs }}
{{if .Endpoint}}Endpoint = {{ .Endpoint }}{{end}}
{{if .PersistentKeepalive}}PersistentKeepalive = {{ .PersistentKeepalive }}{{end}}
{{end}}
`

func RenderTemplate(settings Config) (string, error) {
	tmpl, parseErr := template.New("greeting").Parse(theTemplate)
	if parseErr != nil {
		return "", parseErr
	}

	var buffer bytes.Buffer
	executeErr := tmpl.Execute(&buffer, settings)
	if executeErr != nil {
		return "", executeErr
	}

	return buffer.String(), nil
}

type Watcher struct {
	path                string
	fs                  afero.Fs
	lastWrittenTemplate string
}

func (w *Watcher) Update(settings Config) error {
	content, renderErr := RenderTemplate(settings)
	if renderErr != nil {
		return renderErr
	}
	if sha256Hash(content) == sha256Hash(w.lastWrittenTemplate) {
		return nil
	}

	writeErr := afero.WriteFile(w.fs, w.path, []byte(content), 0600)
	if writeErr != nil {
		return writeErr
	}
	w.lastWrittenTemplate = content
	return nil
}

func NewWatcher(cfgPath string) *Watcher {
	fs := &afero.Afero{Fs: afero.NewOsFs()}
	createErr := fs.MkdirAll(filepath.Dir(cfgPath), 0700)
	if createErr != nil && createErr != afero.ErrDestinationExists {
		logrus.Panicf("Could not create Wireguard config directory at %s: %v", cfgPath, createErr)
	}
	return &Watcher{
		path: cfgPath,
		fs:   fs,
	}
}

func sha256Hash(i string) string {
	hash := sha256.New()
	hash.Write([]byte(i))
	return hex.EncodeToString(hash.Sum(nil))
}
