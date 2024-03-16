package events

import (
	"fmt"
)

func RemoteSessionStartedTopic(id string) string {
	return fmt.Sprintf("session::remote::%s::started", id)
}

func LocalSessionStartedTopic(sessionID, appName, peerName string) string {
	return fmt.Sprintf("session::local::%s::%s::%s::started", peerName, appName, sessionID)
}

func RemoteSessionClientDataSentTopic(sessionID string, appName string) string {
	return fmt.Sprintf("session::remote::%s::client::%s::data::sent", sessionID, appName)
}

func RemoteSessionAppDataSentTopic(sessionID string, appName string) string {
	return fmt.Sprintf("session::remote::%s::app::%s::data::sent", sessionID, appName)
}
func LocalSessionAppDataSentTopic(sessionID string, appName string) string {
	return fmt.Sprintf("session::local::%s::app::%s::data::sent", sessionID, appName)
}

func RemoteSessionAppEOFTopic(sessionID string, appName string) string {
	return fmt.Sprintf("session::remote::%s::app::%s::eof", sessionID, appName)
}

func LocalSessionAppEOFTopic(sessionID string, appName string) string {
	return fmt.Sprintf("session::local::%s::app::%s::eof", sessionID, appName)
}

func RemoteSessionClientDisconnectedTopic(id string) string {
	return fmt.Sprintf("session::remote::%s::client::disconnected", id)
}

func RemoteAppExposedTopic(peerName string) string {
	return fmt.Sprintf("app::remote::%s::exposed", peerName)
}

func RemoteAppWithdrawnTopic(peerName string) string {
	return fmt.Sprintf("app::remote::%s::withdrawn", peerName)
}

func RemoteSessionServerDisconnectedTopic(id string) string {
	return fmt.Sprintf("session::remote::%s::server::disconnected", id)
}

func RemoteSessionFinishedTopic(id string) string {
	return fmt.Sprintf("session::remote::%s::finished", id)
}

const (
	LocalAppExposedTopic   = "app::local::exposed"
	LocalAppWithdrawnTopic = "app::local::withdrawn"
	PeerConnected          = "peer::connected"
	PeerDisconnected       = "peer::disconnected"
	PingTopic              = "ping"
)
