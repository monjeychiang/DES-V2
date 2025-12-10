// Package balance provides multi-user balance management.
package balance

import (
	"context"
	"sync"
)

// MultiUserManager manages balances for multiple users.
type MultiUserManager struct {
	mu       sync.RWMutex
	managers map[string]*Manager // userID -> Manager
	factory  ManagerFactory
}

// ManagerFactory creates a Manager for a user.
type ManagerFactory func(userID string) (*Manager, error)

// NewMultiUserManager creates a new multi-user balance manager.
func NewMultiUserManager(factory ManagerFactory) *MultiUserManager {
	return &MultiUserManager{
		managers: make(map[string]*Manager),
		factory:  factory,
	}
}

// GetOrCreate returns the balance manager for a user, creating if needed.
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
	mgr, err := m.factory(userID)
	if err != nil {
		return nil, err
	}

	m.managers[userID] = mgr
	return mgr, nil
}

// Get returns the balance manager for a user, or nil if not found.
func (m *MultiUserManager) Get(userID string) *Manager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.managers[userID]
}

// Remove removes the balance manager for a user.
func (m *MultiUserManager) Remove(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.managers, userID)
}

// StartAll starts all user managers.
func (m *MultiUserManager) StartAll(ctx context.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, mgr := range m.managers {
		mgr.Start(ctx)
	}
}

// GetAllBalances returns balances for all users.
func (m *MultiUserManager) GetAllBalances() map[string]Balance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]Balance)
	for userID, mgr := range m.managers {
		result[userID] = mgr.GetBalance()
	}
	return result
}

// UserCount returns the number of active user managers.
func (m *MultiUserManager) UserCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.managers)
}
