package api

import (
	"github.com/gin-gonic/gin"
	"github.com/glothriel/wormhole/pkg/pairing"
	"github.com/glothriel/wormhole/pkg/syncing"
	"github.com/glothriel/wormhole/pkg/wg"
)

// PeerController is a controller for managing peers
type PeerController struct {
	peers    pairing.PeerStorage
	wgConfig *wg.Config
	watcher  *wg.Watcher

	metadata           syncing.MetadataStorage
	enablePeerDeletion bool
}

func (p *PeerController) deletePeer(name string) error {
	peerInfo, err := p.peers.GetByName(name)
	if err != nil {
		return err
	}
	p.wgConfig.DeleteByPublicKey(peerInfo.PublicKey)
	err = p.watcher.Update(*p.wgConfig)
	if err != nil {
		return err
	}
	err = p.peers.DeleteByName(name)
	if err != nil {
		return err
	}
	return nil
}

// PeersV2ListItem is a struct for the v2 peers list
type PeersV2ListItem struct {
	Name     string           `json:"name"`
	Metadata syncing.Metadata `json:"metadata"`
}

func (p *PeerController) registerRoutes(r *gin.Engine) {
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

	r.GET("/api/peers/v2", func(c *gin.Context) {
		peerList, err := p.peers.List()
		if err != nil {
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}
		var peerListItems []PeersV2ListItem
		for _, peer := range peerList {
			metadata, err := p.metadata.Get(peer.Name)
			if err != nil {
				c.JSON(500, gin.H{
					"error": err.Error(),
				})
				return
			}
			peerListItems = append(peerListItems, PeersV2ListItem{
				Name:     peer.Name,
				Metadata: metadata,
			})
		}
		c.JSON(200, peerListItems)
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

// PeerControllerSettings is a type for setting up the PeerController
type PeerControllerSettings func(*PeerController)

// EnablePeerDeletion enables peer deletion
func EnablePeerDeletion(controller *PeerController) {
	controller.enablePeerDeletion = true
}

// NewPeersController allows querying and manipulation of the connected peers
func NewPeersController(
	peers pairing.PeerStorage,
	wgConfig *wg.Config,
	watcher *wg.Watcher,
	metadata syncing.MetadataStorage,
	settings ...PeerControllerSettings,
) Controller {
	theController := &PeerController{
		peers:    peers,
		wgConfig: wgConfig,
		watcher:  watcher,
		metadata: metadata,
		// We currently don't have authorization in place, disabling peer deletion
		enablePeerDeletion: false,
	}
	for _, setting := range settings {
		setting(theController)
	}
	return theController
}
