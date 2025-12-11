package api

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"trading-core/internal/monitor"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// Per-IP rate limiters
var (
	ipLimiters = make(map[string]*rate.Limiter)
	mu         sync.RWMutex
)

func getIPLimiter(ip string) *rate.Limiter {
	mu.RLock()
	limiter, exists := ipLimiters[ip]
	mu.RUnlock()

	if exists {
		return limiter
	}

	mu.Lock()
	defer mu.Unlock()

	// Check again in case another goroutine created it
	if limiter, exists := ipLimiters[ip]; exists {
		return limiter
	}

	// Create new limiter: 20 req/s per IP, burst 50
	limiter = rate.NewLimiter(rate.Limit(20), 50)
	ipLimiters[ip] = limiter
	return limiter
}

// Cleanup old limiters periodically
func init() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			ipLimiters = make(map[string]*rate.Limiter)
			mu.Unlock()
		}
	}()
}

// CORSMiddleware handles Cross-Origin Resource Sharing
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// RequestIDMiddleware adds unique request ID for tracking
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set("RequestID", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
	}
}

// RateLimitMiddleware prevents API abuse with per-IP rate limiting
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := getIPLimiter(ip)

		if !limiter.Allow() {
			log.Printf("[RATE_LIMIT] IP %s exceeded rate limit", ip)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate limit exceeded",
				"message": "too many requests, please slow down",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// TimeoutMiddleware prevents long-running requests from blocking resources
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		finished := make(chan struct{})
		panicChan := make(chan interface{}, 1)

		go func() {
			defer func() {
				if p := recover(); p != nil {
					panicChan <- p
				}
			}()
			c.Next()
			finished <- struct{}{}
		}()

		select {
		case <-panicChan:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
			c.Abort()
		case <-finished:
			return
		case <-ctx.Done():
			log.Printf("[TIMEOUT] Request timeout: %s %s", c.Request.Method, c.Request.URL.Path)
			c.JSON(http.StatusRequestTimeout, gin.H{
				"error":   "request timeout",
				"message": "request took too long to process",
			})
			c.Abort()
		}
	}
}

// RequestLogger logs all API requests with timing and status; optionally records metrics.
func RequestLogger(metrics *monitor.SystemMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method
		requestID := c.GetString("RequestID")
		if requestID == "" {
			requestID = "unknown"
		}

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()

		if metrics != nil {
			metrics.IncrementAPI()
			metrics.APILatency.RecordDuration(latency)
			if statusCode >= 400 {
				metrics.IncrementAPIErrors()
			}
		}

		// Log request
		log.Printf("[API] %s | %s %s | %d | %v | %s",
			requestID[:8], // Show first 8 chars of UUID
			method,
			path,
			statusCode,
			latency,
			clientIP,
		)
	}
}
