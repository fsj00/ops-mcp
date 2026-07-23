package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// requireAuth enforces Bearer / X-API-Key token when configured.
// Public paths: GET /health, GET / and embedded static assets under web.FS.
// Empty configured token disables auth.
func (s *Server) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := s.cfg.AuthToken()
		if expected == "" {
			c.Next()
			return
		}
		if isPublicPath(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}
		got := extractToken(c)
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
			})
			return
		}
		c.Next()
	}
}

// isPublicPath reports whether method+path may be served without a token.
// Only GET/HEAD for /health and embedded UI paths are public; /api and /mcp stay protected.
func isPublicPath(method, path string) bool {
	if method != http.MethodGet && method != http.MethodHead {
		return false
	}
	switch path {
	case "/health", "/", "/index.html":
		return true
	default:
		return false
	}
}

func extractToken(c *gin.Context) string {
	if auth := c.GetHeader("Authorization"); auth != "" {
		const prefix = "Bearer "
		if strings.HasPrefix(auth, prefix) {
			return strings.TrimSpace(auth[len(prefix):])
		}
		return strings.TrimSpace(auth)
	}
	if key := c.GetHeader("X-API-Key"); key != "" {
		return strings.TrimSpace(key)
	}
	if key := c.Query("token"); key != "" {
		return strings.TrimSpace(key)
	}
	return ""
}
