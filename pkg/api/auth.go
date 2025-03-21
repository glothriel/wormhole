package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireBasicAuth middleware checks if basic auth is properly configured
// and validates credentials when present
func RequireBasicAuth(s ServerSettings) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.BasicAuthEnabled {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Server misconfiguration: Basic authentication is required but not enabled",
			})
			return
		}

		username, password, hasAuth := c.Request.BasicAuth()
		if !hasAuth || username != s.BasicAuthUsername || password != s.BasicAuthPassword {
			c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized: Invalid credentials",
			})
			return
		}

		c.Next()
	}
}
