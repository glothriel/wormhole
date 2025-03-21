package cmd

import "github.com/urfave/cli/v2"

var nginxExposerConfdPathFlag *cli.StringFlag = &cli.StringFlag{
	Name:  "nginx-confd-path",
	Value: "/storage/nginx",
}

var wireguardConfigFilePathFlag *cli.StringFlag = &cli.StringFlag{
	Name:  "wg-config",
	Value: "/storage/wireguard/wg0.conf",
}

var peerStorageDBFlag *cli.StringFlag = &cli.StringFlag{
	Name:  "peer-storage-db",
	Value: "",
}

var peerMetadataStorageDBFlag *cli.StringFlag = &cli.StringFlag{
	Name:  "peer-metadata-storage-db",
	Value: "",
}

var clientMetadataFlag *cli.StringFlag = &cli.StringFlag{
	Name:    "client-metadata",
	Value:   "{}",
	EnvVars: []string{"CLIENT_METADATA"},
	Usage:   "JSON-formatted metadata to send to the server with every sync request",
}

var keyStorageDBFlag *cli.StringFlag = &cli.StringFlag{
	Name:  "key-storage-db",
	Value: "",
}

var pairingClientCacheDBPath *cli.StringFlag = &cli.StringFlag{
	Name:  "pairing-client-cache-db",
	Value: "",
}

var kubernetesFlag *cli.BoolFlag = &cli.BoolFlag{
	Name:  "kubernetes",
	Usage: "Use kubernetes to create proxy services",
}

var kubernetesNamespaceFlag *cli.StringFlag = &cli.StringFlag{
	Name:  "kubernetes-namespace",
	Value: "",
	Usage: "Namespace to create the proxy services in",
}

var kubernetesLabelsFlag *cli.StringFlag = &cli.StringFlag{
	Name:  "kubernetes-labels",
	Value: "",
	Usage: ("Labels that will be set on proxy service, must match the labels of wormhole server pod. " +
		"Format: key1=value1,key2=value2"),
}

var stateManagerPathFlag *cli.StringFlag = &cli.StringFlag{
	Name:   "directory-state-manager-path",
	Hidden: true,
	Value:  "",
}

var inviteTokenFlag *cli.StringFlag = &cli.StringFlag{
	Name:    "invite-token",
	Usage:   "Invite token to use to connect to the wormhole server",
	EnvVars: []string{"INVITE_TOKEN"},
	Value:   "",
}

var peerNameFlag *cli.StringFlag = &cli.StringFlag{
	Name:     "name",
	Required: true,
}

var enableNetworkPoliciesFlag *cli.BoolFlag = &cli.BoolFlag{
	Name:  "network-policies",
	Usage: "Enables dynamic creation of network policies for proxy services",
	Value: false,
}

var basicAuthUsernameFlag *cli.StringFlag = &cli.StringFlag{
	Name:    "basic-auth-username",
	Usage:   "Basic auth username, used only for subset (state changing) of endpoints",
	EnvVars: []string{"BASIC_AUTH_USERNAME"},
	Value:   "",
}

var basicAuthPasswordFlag *cli.StringFlag = &cli.StringFlag{
	Name:    "basic-auth-password",
	Usage:   "Basic auth password, used only for subset (state changing) of endpoints",
	EnvVars: []string{"BASIC_AUTH_PASSWORD"},
	Value:   "",
}
