package peers

type App struct {
	Name    string
	Peer    string
	Address string
}

type AppSource interface {
	Changed() chan []App
}

type AppExposer interface {
	Expose([]App)
}
