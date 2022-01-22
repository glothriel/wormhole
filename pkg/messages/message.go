package messages

// Message represents a packet that is transmitted between the peers
type Message struct {
	SessionID  string
	AppName    string
	Type       string
	BodyString string
}

// Body extracts the payload from the message
func Body(m Message) []byte {
	return []byte(m.BodyString)
}

// WithAppName returns a copy of a message, with its app name modified
func WithAppName(m Message, name string) Message {
	return Message{
		SessionID:  m.SessionID,
		Type:       m.Type,
		BodyString: m.BodyString,
		AppName:    name,
	}
}
