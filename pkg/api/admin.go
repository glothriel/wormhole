// Package api contains administrative APIs used for querying and manipulation of peers and apps
package api

import (
	"github.com/gin-gonic/gin"

	"github.com/gin-contrib/pprof"
)

// Controller contains a set of functionalities for the API
type Controller interface {
	registerRoutes(r *gin.Engine, s ServerSettings)
}

// ServerSettings contains the settings for the server
type ServerSettings struct {
	Debug             bool
	BasicAuthEnabled  bool
	BasicAuthUsername string
	BasicAuthPassword string
}

// NewServerSettings creates a new server settings object
func NewServerSettings() ServerSettings {
	return ServerSettings{}
}

// WithDebug sets the debug flag
func (ss ServerSettings) WithDebug(debug bool) ServerSettings {
	ss.Debug = debug
	return ss
}

// WithBasicAuth allows configuration of basic auth
func (ss ServerSettings) WithBasicAuth(username, password string) ServerSettings {
	ss.BasicAuthEnabled = true
	ss.BasicAuthUsername = username
	ss.BasicAuthPassword = password
	return ss
}

// NewAdminAPI bootstraps the creation of the gin engine
func NewAdminAPI(controllers []Controller, settings ServerSettings) *gin.Engine {
	r := gin.Default()
	for _, controller := range controllers {
		controller.registerRoutes(r, settings)
	}
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Wormholes are allowable within the laws of physics, but there's no observational evidence" +
				" for them. If you find a wormhole, you're a very lucky person because they are extremely rare.",
		})
	})

	if settings.Debug {
		pprof.Register(r)
	}
	return r
}
