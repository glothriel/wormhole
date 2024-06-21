package api

import (
	"github.com/gin-gonic/gin"
	"github.com/glothriel/wormhole/pkg/hello"
	"github.com/glothriel/wormhole/pkg/peers"
)

type appsController struct {
	appSource hello.AppSource
}

func (ac *appsController) registerRoutes(r *gin.Engine) {
	r.GET("/api/apps/v1", func(c *gin.Context) {
		apps, err := ac.appSource.List()
		if err != nil {
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}
		if apps == nil {
			apps = []peers.App{}
		}
		c.JSON(200, apps)
	})
}

// NewAppsController bootstraps creation of the API that allows displaying currently exposed apps
func NewAppsController(appSource hello.AppSource) Controller {
	return &appsController{appSource: appSource}
}
