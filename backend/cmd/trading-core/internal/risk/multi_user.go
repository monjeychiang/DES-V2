// Package risk provides multi-user risk management.
package risk

import (
	"context"
	"database/sql"
	"sync"
)

// MultiUserManager manages risk managers for multiple users.
type MultiUserManager struct {
	mu       sync.RWMutex
	managers map[string]*Manager // userID -> Manager
	db       *sql.DB
}

// NewMultiUserManager creates a new multi-user risk manager.
func NewMultiUserManager(db *sql.DB) *MultiUserManager {
	return &MultiUserManager{
		managers: make(map[string]*Manager),
		db:       db,
	}
}

// GetOrCreate returns the risk manager for a user, creating if needed.
func (m *MultiUserManager) GetOrCreate(userID string) (*Manager, error) {
	m.mu.RLock()
	if mgr, ok := m.managers[userID]; ok {
		m.mu.RUnlock()
		return mgr, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check
	if mgr, ok := m.managers[userID]; ok {
		return mgr, nil
	}

	// Create new manager
	// For now, use in-memory with default config
	// TODO: load per-user config from DB
	mgr := NewInMemory(DefaultConfig())
	m.managers[userID] = mgr
	return mgr, nil
}

// Get returns the risk manager for a user, or nil if not found.
func (m *MultiUserManager) Get(userID string) *Manager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.managers[userID]
}

// Remove removes the risk manager for a user.
func (m *MultiUserManager) Remove(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.managers, userID)
}

// GetAllMetrics returns risk metrics for all users.
func (m *MultiUserManager) GetAllMetrics() map[string]RiskMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]RiskMetrics)
	for userID, mgr := range m.managers {
		result[userID] = mgr.GetMetrics()
	}
	return result
}

// UserCount returns the number of active user managers.
func (m *MultiUserManager) UserCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.managers)
}

// UpdateMetricsForUser updates metrics for a specific user.
func (m *MultiUserManager) UpdateMetricsForUser(ctx context.Context, userID string, trade TradeResult) error {
	mgr, err := m.GetOrCreate(userID)
	if err != nil {
		return err
	}
	return mgr.UpdateMetrics(trade)
}

// ResetDailyForAll resets daily metrics for all users.
func (m *MultiUserManager) ResetDailyForAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, mgr := range m.managers {
		mgr.ResetDailyMetrics()
	}
}

// EvaluateForUser evaluates a signal for a specific user.
func (m *MultiUserManager) EvaluateForUser(userID string, signal SignalInput, position Position, account Account, strategyID string) (RiskDecision, error) {
	mgr, err := m.GetOrCreate(userID)
	if err != nil {
		return RiskDecision{Allowed: false, Reason: "failed to get risk manager"}, err
	}
	return mgr.EvaluateFull(signal, position, account, strategyID), nil
}
