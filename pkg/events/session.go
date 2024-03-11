package events

import "fmt"

func SessionStartedTopic(id string) string {
	return fmt.Sprintf("session::%s::started", id)
}

func SessionClientDataSentTopic(sessionID string, appName string) string {
	return fmt.Sprintf("session::%s::client::%s::data::sent", sessionID, appName)
}

func SessionAppDataSentTopic(sessionID string, appName string) string {
	return fmt.Sprintf("session::%s::app::%s::data::sent", sessionID, appName)
}

func SessionClientDisconnectedTopic(id string) string {
	return fmt.Sprintf("session::%s::client::disconnected", id)
}

func SessionServerDisconnectedTopic(id string) string {
	return fmt.Sprintf("session::%s::server::disconnected", id)
}

func SessionFinishedTopic(id string) string {
	return fmt.Sprintf("session::%s::finished", id)
}

const (
	LocalAppExposedTopic   = "app::exposed"
	LocalAppWithdrawnTopic = "app::withdrawn"
	PeerDisconnected       = "peer::disconnected"
	PingTopic              = "ping"
)
