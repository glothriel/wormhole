package peers

type App struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Peer    string `json:"peer"`

	OriginalPort int32  `json:"originalPort"`
	TargetLabels string `json:"targetLabels"`
}

type AppSource interface {
	Changed() chan []App
}

type AppExposer interface {
	Expose([]App)
}

func WithAddress(app App, newAddress string) App {
	a := app
	a.Address = newAddress
	return a
}

func WithPeer(app App, newPeer string) App {
	a := app
	a.Peer = newPeer
	return a
}
