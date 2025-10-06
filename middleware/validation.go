package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequestValidationMiddleware validates common request parameters
func RequestValidationMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Check Content-Type for POST, PUT, PATCH requests
		method := c.Request.Method
		if method == "POST" || method == "PUT" || method == "PATCH" {
			contentType := c.GetHeader("Content-Type")

			// Allow multipart/form-data for file uploads
			if !strings.Contains(contentType, "application/json") &&
				!strings.Contains(contentType, "multipart/form-data") {
				c.JSON(http.StatusUnsupportedMediaType, gin.H{
					"error": "Content-Type must be application/json or multipart/form-data",
				})
				c.Abort()
				return
			}
		}

		// Validate Accept header if present
		accept := c.GetHeader("Accept")
		if accept != "" && !strings.Contains(accept, "application/json") &&
			!strings.Contains(accept, "*/*") {
			c.JSON(http.StatusNotAcceptable, gin.H{
				"error": "API only supports application/json responses",
			})
			c.Abort()
			return
		}

		// Check for suspicious patterns in User-Agent
		userAgent := c.GetHeader("User-Agent")
		suspiciousPatterns := []string{"sqlmap", "nmap", "nikto", "scanner", "crawler"}
		for _, pattern := range suspiciousPatterns {
			if strings.Contains(strings.ToLower(userAgent), pattern) {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Request rejected",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	})
}
