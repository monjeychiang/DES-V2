package balance

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ExchangeClient interface for getting balance
type ExchangeClient interface {
	GetBalance(ctx context.Context) (Balance, error)
}

// Balance represents account balance
type Balance struct {
	Total     float64
	Available float64
	Locked    float64
}

// Manager manages account balance
type Manager struct {
	exchange     ExchangeClient
	cache        *BalanceCache
	syncInterval time.Duration
	mu           sync.RWMutex
}

// BalanceCache caches balance data
type BalanceCache struct {
	total     float64
	available float64
	locked    float64
	lastSync  time.Time
	mu        sync.RWMutex
}

// NewManager creates a new balance manager
func NewManager(exchange ExchangeClient, syncInterval time.Duration) *Manager {
	return &Manager{
		exchange:     exchange,
		cache:        &BalanceCache{},
		syncInterval: syncInterval,
	}
}

// Start begins periodic balance sync
func (m *Manager) Start(ctx context.Context) {
	// Initial sync
	m.Sync(ctx)

	ticker := time.NewTicker(m.syncInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := m.Sync(ctx); err != nil {
					log.Printf("❌ Balance sync error: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Sync fetches latest balance from exchange
func (m *Manager) Sync(ctx context.Context) error {
	if m.exchange == nil {
		// No exchange configured (dry-run mode)
		return nil
	}

	balance, err := m.exchange.GetBalance(ctx)
	if err != nil {
		return err
	}

	m.cache.mu.Lock()
	m.cache.total = balance.Total
	m.cache.available = balance.Available
	m.cache.locked = balance.Locked
	m.cache.lastSync = time.Now()
	m.cache.mu.Unlock()

	log.Printf("💰 Balance synced: Total=%.2f, Available=%.2f, Locked=%.2f",
		balance.Total, balance.Available, balance.Locked)

	return nil
}

// GetAvailable returns available balance
func (m *Manager) GetAvailable() float64 {
	m.cache.mu.RLock()
	defer m.cache.mu.RUnlock()
	return m.cache.available
}

// Lock reserves balance for order
func (m *Manager) Lock(amount float64) error {
	m.cache.mu.Lock()
	defer m.cache.mu.Unlock()

	if amount > m.cache.available {
		return fmt.Errorf("insufficient balance: need %.2f, have %.2f",
			amount, m.cache.available)
	}

	m.cache.available -= amount
	m.cache.locked += amount

	log.Printf("🔒 Balance locked: %.2f (Available: %.2f)", amount, m.cache.available)
	return nil
}

// Unlock releases locked balance
func (m *Manager) Unlock(amount float64) {
	m.cache.mu.Lock()
	defer m.cache.mu.Unlock()

	m.cache.locked -= amount
	m.cache.available += amount

	log.Printf("🔓 Balance unlocked: %.2f (Available: %.2f)", amount, m.cache.available)
}

// Deduct removes balance after order filled
func (m *Manager) Deduct(amount float64) {
	m.cache.mu.Lock()
	defer m.cache.mu.Unlock()

	m.cache.locked -= amount
	m.cache.total -= amount

	log.Printf("💸 Balance deducted: %.2f (Total: %.2f)", amount, m.cache.total)
}

// Add adds balance (for sell orders)
func (m *Manager) Add(amount float64) {
	m.cache.mu.Lock()
	defer m.cache.mu.Unlock()

	m.cache.total += amount
	m.cache.available += amount

	log.Printf("💵 Balance added: %.2f (Total: %.2f)", amount, m.cache.total)
}

// GetBalance returns current balance snapshot
func (m *Manager) GetBalance() Balance {
	m.cache.mu.RLock()
	defer m.cache.mu.RUnlock()

	return Balance{
		Total:     m.cache.total,
		Available: m.cache.available,
		Locked:    m.cache.locked,
	}
}

// SetInitialBalance sets initial balance (for dry-run mode)
func (m *Manager) SetInitialBalance(amount float64) {
	m.cache.mu.Lock()
	defer m.cache.mu.Unlock()

	m.cache.total = amount
	m.cache.available = amount
	m.cache.locked = 0

	log.Printf("💰 Initial balance set: %.2f", amount)
}
