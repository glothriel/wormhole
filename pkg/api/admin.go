// Package api contains administrative APIs used for querying and manipulation of peers and apps
package api

import (
	"github.com/gin-gonic/gin"
)

// Controller contains a set of functionalities for the API
type Controller interface {
	registerRoutes(r *gin.Engine)
}

// NewAdminAPI bootstraps the creation of the gin engine
func NewAdminAPI(controllers []Controller) *gin.Engine {
	r := gin.Default()
	for _, controller := range controllers {
		controller.registerRoutes(r)
	}
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Wormholes are allowable within the laws of physics, but there's no observational evidence" +
				" for them. If you find a wormhole, you're a very lucky person because they are extremely rare.",
		})
	})
	return r
}
