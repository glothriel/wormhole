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
