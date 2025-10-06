package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")

		// XSS Protection
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")

		// Prevent clickjacking
		c.Writer.Header().Set("X-Frame-Options", "DENY")

		// HSTS (only in production with HTTPS)
		// c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// CSP Header
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'")

		// Referrer Policy
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Remove server information
		c.Writer.Header().Set("Server", "")

		c.Next()
	})
}
