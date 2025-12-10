// Package gateway provides multi-connection Gateway management for multi-user architecture.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"trading-core/pkg/crypto"
	"trading-core/pkg/db"
	exchange "trading-core/pkg/exchanges/common"
)

var (
	ErrConnectionNotFound = errors.New("connection not found")
	ErrGatewayUnhealthy   = errors.New("gateway is unhealthy")
	ErrPoolFull           = errors.New("gateway pool is full")
)

// GatewayFactory creates a Gateway instance from a Connection.
type GatewayFactory func(conn db.Connection, apiKey, apiSecret string) (exchange.Gateway, error)

// CachedGateway holds a Gateway with metadata for lifecycle management.
type CachedGateway struct {
	Gateway      exchange.Gateway
	ConnectionID string
	UserID       string
	ExchangeType string
	CreatedAt    time.Time
	LastUsed     time.Time
	HealthyAt    time.Time
	Failures     int
}

// Config holds configuration for the GatewayManager.
type Config struct {
	MaxSize          int           // Maximum number of cached gateways (LRU eviction)
	IdleTimeout      time.Duration // Time before idle gateway is removed
	HealthInterval   time.Duration // Interval between health checks
	FailureThreshold int           // Number of failures before marking unhealthy
	CircuitTimeout   time.Duration // Time to wait before retrying unhealthy gateway
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		MaxSize:          100,
		IdleTimeout:      30 * time.Minute,
		HealthInterval:   5 * time.Minute,
		FailureThreshold: 3,
		CircuitTimeout:   5 * time.Minute,
	}
}

// Manager manages a pool of Gateway instances with LRU eviction and health checks.
type Manager struct {
	mu       sync.RWMutex
	gateways map[string]*CachedGateway // connectionID -> cached gateway
	lruOrder []string                  // LRU tracking (oldest first)

	config  Config
	crypto  *crypto.KeyManager
	queries *db.UserQueries
	factory GatewayFactory

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewManager creates a new GatewayManager.
func NewManager(queries *db.UserQueries, cryptoMgr *crypto.KeyManager, factory GatewayFactory, cfg Config) *Manager {
	return &Manager{
		gateways: make(map[string]*CachedGateway),
		lruOrder: make([]string, 0),
		config:   cfg,
		crypto:   cryptoMgr,
		queries:  queries,
		factory:  factory,
		stopCh:   make(chan struct{}),
	}
}

// Start begins background cleanup and health check goroutines.
func (m *Manager) Start(ctx context.Context) {
	m.wg.Add(2)

	// Cleanup goroutine
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(m.config.IdleTimeout / 2)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.cleanupIdle()
			}
		}
	}()

	// Health check goroutine
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(m.config.HealthInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.healthCheckAll()
			}
		}
	}()
}

// Stop gracefully shuts down the manager.
func (m *Manager) Stop() {
	close(m.stopCh)
	m.wg.Wait()

	// Close all gateways
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cached := range m.gateways {
		if closer, ok := cached.Gateway.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		delete(m.gateways, id)
	}
	m.lruOrder = nil
}

// GetOrCreate returns an existing Gateway or creates a new one.
func (m *Manager) GetOrCreate(ctx context.Context, userID, connectionID string) (exchange.Gateway, error) {
	// Fast path: check if already cached
	m.mu.RLock()
	if cached, ok := m.gateways[connectionID]; ok {
		// Verify ownership
		if cached.UserID != userID {
			m.mu.RUnlock()
			return nil, ErrConnectionNotFound
		}
		// Check circuit breaker
		if cached.Failures >= m.config.FailureThreshold {
			if time.Since(cached.HealthyAt) < m.config.CircuitTimeout {
				m.mu.RUnlock()
				return nil, ErrGatewayUnhealthy
			}
		}
		m.mu.RUnlock()

		// Update LRU
		m.touchLRU(connectionID)
		return cached.Gateway, nil
	}
	m.mu.RUnlock()

	// Slow path: need to create
	return m.createGateway(ctx, userID, connectionID)
}

// createGateway creates a new Gateway instance from database.
func (m *Manager) createGateway(ctx context.Context, userID, connectionID string) (exchange.Gateway, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring lock
	if cached, ok := m.gateways[connectionID]; ok {
		if cached.UserID != userID {
			return nil, ErrConnectionNotFound
		}
		m.touchLRULocked(connectionID)
		return cached.Gateway, nil
	}

	// Check pool size
	if len(m.gateways) >= m.config.MaxSize {
		// Evict oldest
		if !m.evictOldestLocked() {
			return nil, ErrPoolFull
		}
	}

	// Fetch connection from database
	conn, err := m.queries.GetConnectionByID(ctx, userID, connectionID)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	if conn == nil {
		return nil, ErrConnectionNotFound
	}

	// Decrypt API keys
	var apiKey, apiSecret string
	if conn.APIKeyEncrypted != "" && m.crypto != nil {
		apiKey, err = m.crypto.Decrypt(conn.APIKeyEncrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt api key: %w", err)
		}
		apiSecret, err = m.crypto.Decrypt(conn.APISecretEncrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt api secret: %w", err)
		}
	} else {
		// Fallback to plaintext (legacy)
		apiKey = conn.APIKey
		apiSecret = conn.APISecret
	}

	// Create gateway using factory
	gw, err := m.factory(*conn, apiKey, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("create gateway: %w", err)
	}

	// Cache it
	now := time.Now()
	m.gateways[connectionID] = &CachedGateway{
		Gateway:      gw,
		ConnectionID: connectionID,
		UserID:       userID,
		ExchangeType: conn.ExchangeType,
		CreatedAt:    now,
		LastUsed:     now,
		HealthyAt:    now,
		Failures:     0,
	}
	m.lruOrder = append(m.lruOrder, connectionID)

	return gw, nil
}

// Remove removes a gateway from the pool.
func (m *Manager) Remove(connectionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cached, ok := m.gateways[connectionID]; ok {
		if closer, ok := cached.Gateway.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		delete(m.gateways, connectionID)
		m.removeLRULocked(connectionID)
	}
}

// RemoveByUser removes all gateways for a user.
func (m *Manager) RemoveByUser(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, cached := range m.gateways {
		if cached.UserID == userID {
			if closer, ok := cached.Gateway.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
			delete(m.gateways, id)
			m.removeLRULocked(id)
		}
	}
}

// RecordFailure records a failure for a gateway.
func (m *Manager) RecordFailure(connectionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cached, ok := m.gateways[connectionID]; ok {
		cached.Failures++
	}
}

// RecordSuccess resets the failure counter.
func (m *Manager) RecordSuccess(connectionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cached, ok := m.gateways[connectionID]; ok {
		cached.Failures = 0
		cached.HealthyAt = time.Now()
	}
}

// Stats returns current pool statistics.
func (m *Manager) Stats() PoolStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := PoolStats{
		TotalGateways:  len(m.gateways),
		MaxSize:        m.config.MaxSize,
		ByExchangeType: make(map[string]int),
		UnhealthyCount: 0,
	}

	for _, cached := range m.gateways {
		stats.ByExchangeType[cached.ExchangeType]++
		if cached.Failures >= m.config.FailureThreshold {
			stats.UnhealthyCount++
		}
	}

	return stats
}

// PoolStats contains gateway pool statistics.
type PoolStats struct {
	TotalGateways  int
	MaxSize        int
	ByExchangeType map[string]int
	UnhealthyCount int
}

// --- Internal helpers ---

func (m *Manager) touchLRU(connectionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.touchLRULocked(connectionID)
}

func (m *Manager) touchLRULocked(connectionID string) {
	// Update last used time
	if cached, ok := m.gateways[connectionID]; ok {
		cached.LastUsed = time.Now()
	}

	// Move to end of LRU list
	for i, id := range m.lruOrder {
		if id == connectionID {
			m.lruOrder = append(m.lruOrder[:i], m.lruOrder[i+1:]...)
			m.lruOrder = append(m.lruOrder, connectionID)
			break
		}
	}
}

func (m *Manager) removeLRULocked(connectionID string) {
	for i, id := range m.lruOrder {
		if id == connectionID {
			m.lruOrder = append(m.lruOrder[:i], m.lruOrder[i+1:]...)
			break
		}
	}
}

func (m *Manager) evictOldestLocked() bool {
	if len(m.lruOrder) == 0 {
		return false
	}

	oldestID := m.lruOrder[0]
	if cached, ok := m.gateways[oldestID]; ok {
		if closer, ok := cached.Gateway.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		delete(m.gateways, oldestID)
	}
	m.lruOrder = m.lruOrder[1:]
	return true
}

func (m *Manager) cleanupIdle() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var toRemove []string

	for id, cached := range m.gateways {
		if now.Sub(cached.LastUsed) > m.config.IdleTimeout {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		if cached, ok := m.gateways[id]; ok {
			if closer, ok := cached.Gateway.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
			delete(m.gateways, id)
			m.removeLRULocked(id)
		}
	}
}

func (m *Manager) healthCheckAll() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.gateways))
	for id := range m.gateways {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		m.healthCheck(id)
	}
}

func (m *Manager) healthCheck(connectionID string) {
	m.mu.RLock()
	cached, ok := m.gateways[connectionID]
	if !ok {
		m.mu.RUnlock()
		return
	}
	gw := cached.Gateway
	m.mu.RUnlock()

	// Try to ping
	if pinger, ok := gw.(interface{ Ping(context.Context) error }); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := pinger.Ping(ctx)
		cancel()

		if err != nil {
			m.RecordFailure(connectionID)
		} else {
			m.RecordSuccess(connectionID)
		}
	}
}
