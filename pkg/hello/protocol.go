package hello

type helloRequest struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

type helloResponse struct {
	Peer      helloResponsePeer `json:"peer"`
	PeerIP    string            `json:"peer_ip"`
	GatewayIP string            `json:"gateway_ip"`
}

type helloResponsePeer struct {
	PublicKey string `json:"public_key"`
	Endpoint  string `json:"endpoint"`
}

type syncRequestAndResponse struct {
	Apps []syncRequestApp `json:"apps"`
}

type syncRequestApp struct {
	Name         string `json:"name"`
	Peer         string `json:"peer"`
	Port         int    `json:"port"`
	OriginalPort int32  `json:"original_port"`
	TargetLabels string `json:"target_labels"`
}
