package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// BasicAuth returns a Gin middleware that protects routes
// with HTTP Basic Authentication.
// Username and password come from config — never hardcoded.
func BasicAuth(username, password string) gin.HandlerFunc {
	expectedUser := []byte(username)
	expectedPass := []byte(password)

	return func(c *gin.Context) {
		u, p, ok := c.Request.BasicAuth()

		// constant-time comparison avoids leaking credential length/content
		// through response timing. Both comparisons always run.
		userMatch := subtle.ConstantTimeCompare([]byte(u), expectedUser) == 1
		passMatch := subtle.ConstantTimeCompare([]byte(p), expectedPass) == 1

		if !ok || !userMatch || !passMatch {
			// WWW-Authenticate header tells the browser/client
			// that basic auth is required
			c.Header("WWW-Authenticate", `Basic realm="notiq-admin"`)
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "unauthorized",
			})
			c.Abort() // stop processing — don't call next handler
			return
		}

		c.Next()
	}
}