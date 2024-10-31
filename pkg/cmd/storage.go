package cmd

import (
	"github.com/glothriel/wormhole/pkg/pairing"
	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/urfave/cli/v2"
)

func getPeerStorage(c *cli.Context) pairing.PeerStorage {
	if c.String(peerStorageDBFlag.Name) == "" {
		return pairing.NewInMemoryPeerStorage()
	}
	return pairing.NewBoltPeerStorage(c.String(peerStorageDBFlag.Name))
}

func getKeyStorage(c *cli.Context) wg.KeyStorage {
	if c.String(keyStorageDBFlag.Name) == "" {
		return wg.NewInMemoryKeyStorage()
	}
	return wg.NewBoltKeyStorage(c.String(keyStorageDBFlag.Name))
}
