package api

import (
	"github.com/gin-gonic/gin"
	"github.com/glothriel/wormhole/pkg/hello"
	"github.com/glothriel/wormhole/pkg/wg"
)

type peerController struct {
	peers              hello.PeerStorage
	wgConfig           *wg.Config
	watcher            *wg.Watcher
	enablePeerDeletion bool
}

func (p *peerController) deletePeer(name string) error {
	peerInfo, err := p.peers.GetByName(name)
	if err != nil {
		return err
	}
	err = p.peers.DeleteByName(name)
	if err != nil {
		return err
	}
	p.wgConfig.DeleteByPublicKey(peerInfo.PublicKey)
	err = p.watcher.Update(*p.wgConfig)
	if err != nil {
		return err
	}
	return nil
}

func (p *peerController) registerRoutes(r *gin.Engine) {
	r.GET("/api/peers/v1", func(c *gin.Context) {
		peerList, err := p.peers.List()
		if err != nil {
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}
		if len(peerList) > 0 {
			c.JSON(200, peerList)
			return
		}
		c.JSON(200, []string{})
	})

	r.DELETE("/api/peers/v1/:name", func(c *gin.Context) {
		if !p.enablePeerDeletion {
			c.JSON(403, gin.H{
				"error": "Peer deletion is disabled",
			})
			return
		}
		name := c.Param("name")
		err := p.deletePeer(name)
		if err != nil {
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(204, nil)
	})
}

// NewPeersController allows querying and manipulation of the connected peers
func NewPeersController(peers hello.PeerStorage, wgConfig *wg.Config, watcher *wg.Watcher) Controller {
	return &peerController{
		peers:    peers,
		wgConfig: wgConfig,
		watcher:  watcher,
		// We currently don't have authorization in place, disabling peer deletion
		enablePeerDeletion: false,
	}
}
