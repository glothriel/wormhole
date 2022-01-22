package messages

const typeData = "data"
const typeDisconnect = "disconnect"
const typePing = "ping"

// IsFrame checks if message contains raw packet data
func IsFrame(m Message) bool {
	return m.Type == typeData
}

// IsDisconnect checks if message is a command to disconnect remote connection
func IsDisconnect(m Message) bool {
	return m.Type == typeDisconnect
}

// IsPing checks if message is heartbeat / ping message used to check conection liveness
func IsPing(m Message) bool {
	return m.Type == typePing
}

// NewFrame Allows creating new message that carries raw packet data
func NewFrame(sessionID string, d []byte) Message {
	return Message{
		SessionID:  sessionID,
		Type:       typeData,
		BodyString: string(d),
	}
}

// NewDisconnect Allows creating new message that carries disconnect command
func NewDisconnect() Message {
	return Message{
		Type: typeDisconnect,
	}
}

// NewPing Allows creating new message that carries ping commans
func NewPing() Message {
	return Message{
		Type: typePing,
	}
}
