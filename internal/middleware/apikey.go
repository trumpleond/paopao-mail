package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"paopao-api/internal/handler"
)

// APIKey protects routes when key is non-empty.
// Accepts header X-API-Key or Authorization: Bearer <key>.
func APIKey(key string) gin.HandlerFunc {
	key = strings.TrimSpace(key)
	return func(c *gin.Context) {
		if key == "" {
			c.Next()
			return
		}
		got := strings.TrimSpace(c.GetHeader("X-API-Key"))
		if got == "" {
			auth := c.GetHeader("Authorization")
			if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				got = strings.TrimSpace(auth[7:])
			}
		}
		if got == "" || got != key {
			handler.Fail(c, http.StatusUnauthorized, 401, "unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}
