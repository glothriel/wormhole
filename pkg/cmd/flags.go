package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"regexp"
	"strings"
)

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

var keyStorageDBFlag *cli.StringFlag = &cli.StringFlag{
	Name:  "key-storage-db",
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
	Usage: "Labels that will be set on proxy service, must match the labels of wormhole server pod. Format: key1=value1,key2=value2",
}

var stateManagerPathFlag *cli.StringFlag = &cli.StringFlag{
	Name:   "directory-state-manager-path",
	Hidden: true,
	Value:  "",
}

var inviteTokenFlag *cli.StringFlag = &cli.StringFlag{
	Name:  "invite-token",
	Usage: "Invite token to use to connect to the wormhole server",
	Value: "",
}

var peerNameFlag *cli.StringFlag = &cli.StringFlag{
	Name:     "name",
	Required: true,
}

func sanitizeStringFlag(context *cli.Context, flagName string) error {
	reg := regexp.MustCompile("[^a-zA-Z0-9]{1}")
	sanitized := reg.ReplaceAllString(context.String(flagName), "-")
	sanitized = strings.ToLower(sanitized)
	logrus.Infof("%s flag set to: %s", flagName, sanitized)
	return context.Set(flagName, sanitized)
}
