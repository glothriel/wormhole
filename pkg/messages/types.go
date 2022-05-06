package messages

const typeFrame = "data"
const typeIntroduction = "introduction"

// TypeAppAdded is message type set when given app is exposed
const TypeAppAdded = "app-added"

// TypeAppWithdrawn is message type set when given app is withdrawn
const TypeAppWithdrawn = "app-withdrawn"
const typeDisconnect = "disconnect"
const typePing = "ping"

const typeSessionClosed = "session-closed"
const typeSessionOpened = "session-opened"

// IsFrame checks if message contains raw packet data
func IsFrame(m Message) bool {
	return m.Type == typeFrame
}

// IsIntroduction checks if message contains peer name
func IsIntroduction(m Message) bool {
	return m.Type == typeIntroduction
}

// IsAppAdded checks if message contains information about added app
func IsAppAdded(m Message) bool {
	return m.Type == TypeAppAdded
}

// IsAppWithdrawn checks if message contains message about withdrawn app
func IsAppWithdrawn(m Message) bool {
	return m.Type == TypeAppWithdrawn
}

// IsDisconnect checks if message is a command to disconnect remote connection
func IsDisconnect(m Message) bool {
	return m.Type == typeDisconnect
}

// IsPing checks if message is heartbeat / ping message used to check conection liveness
func IsPing(m Message) bool {
	return m.Type == typePing
}

// IsSessionOpened checks if the message notifies about opened session
func IsSessionOpened(m Message) bool {
	return m.Type == typeSessionOpened
}

// IsSessionClosed checks if the message notifies about closed session
func IsSessionClosed(m Message) bool {
	return m.Type == typeSessionClosed
}

// NewFrame Allows creating new message that carries raw packet data
func NewFrame(sessionID string, d []byte) Message {
	return Message{
		SessionID:  sessionID,
		Type:       typeFrame,
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

// NewIntroduction allows a peer to introduce to another peer
func NewIntroduction(peerName string) Message {
	return Message{
		Type:       typeIntroduction,
		BodyString: peerName,
	}
}

// NewAppAdded allows adding app added messages
func NewAppAdded(appName string, address string) Message {
	return Message{
		Type:       TypeAppAdded,
		BodyString: AppAddedEncode(appName, address),
	}
}

// NewAppWithdrawn allows creating app withdrawn messages
func NewAppWithdrawn(appName string) Message {
	return Message{
		Type:       TypeAppWithdrawn,
		BodyString: appName,
	}
}

// NewSessionOpened creates new session opened messages
func NewSessionOpened(sessionID string, appName string) Message {
	return Message{
		Type:      typeSessionOpened,
		SessionID: sessionID,
		AppName:   appName,
	}
}

// NewSessionClosed creates new session closed messages
func NewSessionClosed(sessionID string, appName string) Message {
	return Message{
		Type:      typeSessionClosed,
		SessionID: sessionID,
		AppName:   appName,
	}
}
