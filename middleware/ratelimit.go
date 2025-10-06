package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Client represents a rate-limited client
type Client struct {
	tokens   int
	lastSeen time.Time
}

// RateLimiter manages rate limiting
type RateLimiter struct {
	clients map[string]*Client
	mutex   sync.RWMutex
	rate    int           // requests per minute
	burst   int           // maximum burst
	window  time.Duration // time window
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate, burst int) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*Client),
		rate:    rate,
		burst:   burst,
		window:  time.Minute,
	}

	// Cleanup routine
	go rl.cleanup()

	return rl
}

// Allow checks if the client is allowed to make a request
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	client, exists := rl.clients[clientID]

	if !exists {
		client = &Client{
			tokens:   rl.burst - 1,
			lastSeen: now,
		}
		rl.clients[clientID] = client
		return true
	}

	// Calculate tokens to add based on time elapsed
	elapsed := now.Sub(client.lastSeen)
	tokensToAdd := int(elapsed.Seconds() * float64(rl.rate) / 60.0)

	client.tokens = min(client.tokens+tokensToAdd, rl.burst)
	client.lastSeen = now

	if client.tokens > 0 {
		client.tokens--
		return true
	}

	return false
}

// cleanup removes old clients
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute * 10)
	defer ticker.Stop()

	for range ticker.C {
		rl.mutex.Lock()
		now := time.Now()
		for clientID, client := range rl.clients {
			if now.Sub(client.lastSeen) > time.Hour {
				delete(rl.clients, clientID)
			}
		}
		rl.mutex.Unlock()
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RateLimitMiddleware creates a rate limiting middleware
func RateLimitMiddleware(rate, burst int) gin.HandlerFunc {
	limiter := NewRateLimiter(rate, burst)

	return gin.HandlerFunc(func(c *gin.Context) {
		clientID := c.ClientIP()

		if !limiter.Allow(clientID) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": "60s",
			})
			c.Abort()
			return
		}

		c.Next()
	})
}
