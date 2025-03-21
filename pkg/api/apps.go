package api

import (
	"github.com/gin-gonic/gin"
	"github.com/glothriel/wormhole/pkg/apps"
	"github.com/glothriel/wormhole/pkg/syncing"
)

type appsController struct {
	appSource syncing.AppSource
}

func (ac *appsController) registerRoutes(r *gin.Engine, _ ServerSettings) {
	r.GET("/api/apps/v1", func(c *gin.Context) {
		theApps, err := ac.appSource.List()
		if err != nil {
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}
		if theApps == nil {
			theApps = []apps.App{}
		}
		c.JSON(200, theApps)
	})
}

// NewAppsController bootstraps creation of the API that allows displaying currently exposed apps
func NewAppsController(appSource syncing.AppSource) Controller {
	return &appsController{appSource: appSource}
}
