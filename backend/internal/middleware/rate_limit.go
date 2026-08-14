package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateLimiter struct {
	requests  map[string][]time.Time
	mu        sync.Mutex
	limit     int
	window    time.Duration
	cleanupAt time.Time
}

func newRateLimiter(requestsPerMinute int) *rateLimiter {
	return &rateLimiter{
		requests:  make(map[string][]time.Time),
		limit:     requestsPerMinute,
		window:    time.Minute,
		cleanupAt: time.Now().Add(5 * time.Minute),
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Periodic cleanup of old entries
	if now.After(rl.cleanupAt) {
		for key, timestamps := range rl.requests {
			filtered := make([]time.Time, 0)
			for _, t := range timestamps {
				if t.After(cutoff) {
					filtered = append(filtered, t)
				}
			}
			if len(filtered) == 0 {
				delete(rl.requests, key)
			} else {
				rl.requests[key] = filtered
			}
		}
		rl.cleanupAt = now.Add(5 * time.Minute)
	}

	// Get and filter timestamps for this IP
	timestamps := rl.requests[ip]
	filtered := make([]time.Time, 0, len(timestamps))
	for _, t := range timestamps {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}

	// Check if limit exceeded
	if len(filtered) >= rl.limit {
		return false
	}

	// Add current request
	filtered = append(filtered, now)
	rl.requests[ip] = filtered

	return true
}

func RateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
	limiter := newRateLimiter(requestsPerMinute)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !limiter.allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"code":  "RATE_LIMIT_EXCEEDED",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
