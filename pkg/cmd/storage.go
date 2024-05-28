package cmd

import (
	"github.com/glothriel/wormhole/pkg/hello"
	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/urfave/cli/v2"
)

func getPeerStorage(c *cli.Context) hello.PeerStorage {
	if c.String(peerStorageDBFlag.Name) == "" {
		return hello.NewInMemoryPeerStorage()
	}
	return hello.NewBoltPeerStorage(c.String(peerStorageDBFlag.Name))
}

func getKeyStorage(c *cli.Context) wg.KeyStorage {
	if c.String(keyStorageDBFlag.Name) == "" {
		return wg.NewInMemoryKeyStorage()
	}
	return wg.NewBoltKeyStorage(c.String(keyStorageDBFlag.Name))
}
