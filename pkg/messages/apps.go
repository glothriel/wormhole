package messages

// AppStatusChanged is event emmited when application status is changed
type AppStatusChanged struct {
	App    string
	Status string
}
