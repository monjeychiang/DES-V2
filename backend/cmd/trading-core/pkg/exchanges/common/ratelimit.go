package common

import (
	"log"
	"strconv"
	"sync"
	"time"
)

// RateLimiter tracks API rate limit usage.
type RateLimiter struct {
	usedWeight    int
	limit         int
	lastReset     time.Time
	resetInterval time.Duration
	mu            sync.RWMutex
}

// NewRateLimiter creates a new rate limiter.
// limit: maximum weight allowed (e.g., 1200 for spot, 2400 for futures)
// resetInterval: time window (e.g., 1 minute)
func NewRateLimiter(limit int, resetInterval time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:         limit,
		resetInterval: resetInterval,
		lastReset:     time.Now(),
	}
}

// UpdateFromHeader updates the used weight from API response header.
func (rl *RateLimiter) UpdateFromHeader(headerValue string) {
	if headerValue == "" {
		return
	}

	weight, err := strconv.Atoi(headerValue)
	if err != nil {
		return
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Reset if needed
	if time.Since(rl.lastReset) >= rl.resetInterval {
		rl.usedWeight = 0
		rl.lastReset = time.Now()
	}

	rl.usedWeight = weight

	// Warn if approaching limit
	percentage := float64(rl.usedWeight) / float64(rl.limit) * 100
	if percentage >= 95 {
		log.Printf("rate limit critical: %d/%d (%.1f%%) - approaching ban threshold", rl.usedWeight, rl.limit, percentage)
	} else if percentage >= 80 {
		log.Printf("rate limit warning: %d/%d (%.1f%%)", rl.usedWeight, rl.limit, percentage)
	}
}

// GetUsage returns current usage information.
func (rl *RateLimiter) GetUsage() (used int, limit int, percentage float64) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	// Reset if needed
	if time.Since(rl.lastReset) >= rl.resetInterval {
		return 0, rl.limit, 0
	}

	return rl.usedWeight, rl.limit, float64(rl.usedWeight) / float64(rl.limit) * 100
}

// ShouldDelay returns true if we should delay the next request.
func (rl *RateLimiter) ShouldDelay() bool {
	_, _, pct := rl.GetUsage()
	return pct >= 90
}
