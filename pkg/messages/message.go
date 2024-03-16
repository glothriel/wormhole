package messages

import "context"

// Message represents a packet that is transmitted between the peers
type Message struct {
	SessionID  string
	AppName    string
	Type       string
	Context    map[string]string
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
		Context:    m.Context,
		AppName:    name,
	}
}

// WithBody returns a copy of a message, with its body modified
func WithBody(m Message, bodyString string) Message {
	return Message{
		SessionID:  m.SessionID,
		Type:       m.Type,
		BodyString: bodyString,
		Context:    m.Context,
		AppName:    m.AppName,
	}
}

// WithContext returns a copy of a message, with its context modified
func WithContext(m Message, ctx context.Context) Message {
	return Message{
		SessionID:  m.SessionID,
		Type:       m.Type,
		BodyString: m.BodyString,
		Context:    DumpContext(ctx),
		AppName:    m.AppName,
	}
}
