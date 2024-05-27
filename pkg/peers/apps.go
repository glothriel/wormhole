// Package peers defines basic structures for apps and peers
package peers

// App represents an application that can be peered
type App struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Peer    string `json:"peer"`

	OriginalPort int32  `json:"originalPort"`
	TargetLabels string `json:"targetLabels"`
}

// WithAddress returns a new App with the given address
func WithAddress(app App, newAddress string) App {
	a := app
	a.Address = newAddress
	return a
}

// WithPeer returns a new App with the given peer
func WithPeer(app App, newPeer string) App {
	a := app
	a.Peer = newPeer
	return a
}
