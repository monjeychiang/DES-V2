package cache

import (
	"hash/fnv"
	"sync"
	"time"
)

const numShards = 16

// ShardedPriceCache is a high-performance price cache with sharding.
type ShardedPriceCache struct {
	shards [numShards]*priceShard
}

type priceShard struct {
	mu    sync.RWMutex
	items map[string]priceEntry
}

type priceEntry struct {
	price     float64
	updatedAt time.Time
}

// NewShardedPriceCache creates a new sharded cache.
func NewShardedPriceCache() *ShardedPriceCache {
	c := &ShardedPriceCache{}
	for i := 0; i < numShards; i++ {
		c.shards[i] = &priceShard{
			items: make(map[string]priceEntry),
		}
	}
	return c
}

// getShard returns the shard for the given key.
func (c *ShardedPriceCache) getShard(key string) *priceShard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return c.shards[h.Sum32()%numShards]
}

// Set stores a price for a symbol.
func (c *ShardedPriceCache) Set(symbol string, price float64) {
	shard := c.getShard(symbol)
	shard.mu.Lock()
	shard.items[symbol] = priceEntry{
		price:     price,
		updatedAt: time.Now(),
	}
	shard.mu.Unlock()
}

// Get retrieves a price for a symbol.
func (c *ShardedPriceCache) Get(symbol string) (float64, bool) {
	shard := c.getShard(symbol)
	shard.mu.RLock()
	entry, ok := shard.items[symbol]
	shard.mu.RUnlock()
	return entry.price, ok
}

// GetWithAge retrieves price and its age.
func (c *ShardedPriceCache) GetWithAge(symbol string) (float64, time.Duration, bool) {
	shard := c.getShard(symbol)
	shard.mu.RLock()
	entry, ok := shard.items[symbol]
	shard.mu.RUnlock()
	if !ok {
		return 0, 0, false
	}
	return entry.price, time.Since(entry.updatedAt), true
}

// Delete removes a symbol from the cache.
func (c *ShardedPriceCache) Delete(symbol string) {
	shard := c.getShard(symbol)
	shard.mu.Lock()
	delete(shard.items, symbol)
	shard.mu.Unlock()
}

// Len returns total items across all shards.
func (c *ShardedPriceCache) Len() int {
	total := 0
	for _, shard := range c.shards {
		shard.mu.RLock()
		total += len(shard.items)
		shard.mu.RUnlock()
	}
	return total
}

// Cleanup removes entries older than maxAge.
func (c *ShardedPriceCache) Cleanup(maxAge time.Duration) int {
	removed := 0
	cutoff := time.Now().Add(-maxAge)

	for _, shard := range c.shards {
		shard.mu.Lock()
		for sym, entry := range shard.items {
			if entry.updatedAt.Before(cutoff) {
				delete(shard.items, sym)
				removed++
			}
		}
		shard.mu.Unlock()
	}
	return removed
}

// CleanupInvalid removes entries not in validSymbols set.
func (c *ShardedPriceCache) CleanupInvalid(validSymbols []string) int {
	valid := make(map[string]bool, len(validSymbols))
	for _, s := range validSymbols {
		valid[s] = true
	}

	removed := 0
	for _, shard := range c.shards {
		shard.mu.Lock()
		for sym := range shard.items {
			if !valid[sym] {
				delete(shard.items, sym)
				removed++
			}
		}
		shard.mu.Unlock()
	}
	return removed
}

// GetAll returns all cached prices (for debugging/admin).
func (c *ShardedPriceCache) GetAll() map[string]float64 {
	result := make(map[string]float64)
	for _, shard := range c.shards {
		shard.mu.RLock()
		for sym, entry := range shard.items {
			result[sym] = entry.price
		}
		shard.mu.RUnlock()
	}
	return result
}

// CacheStats provides cache statistics.
type CacheStats struct {
	TotalItems  int            `json:"total_items"`
	ShardCounts [numShards]int `json:"shard_counts"`
	OldestAge   time.Duration  `json:"oldest_age"`
}

// Stats returns cache statistics.
func (c *ShardedPriceCache) Stats() CacheStats {
	stats := CacheStats{}
	var oldest time.Time

	for i, shard := range c.shards {
		shard.mu.RLock()
		stats.ShardCounts[i] = len(shard.items)
		stats.TotalItems += len(shard.items)
		for _, entry := range shard.items {
			if oldest.IsZero() || entry.updatedAt.Before(oldest) {
				oldest = entry.updatedAt
			}
		}
		shard.mu.RUnlock()
	}

	if !oldest.IsZero() {
		stats.OldestAge = time.Since(oldest)
	}
	return stats
}
