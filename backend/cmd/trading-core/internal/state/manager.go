package state

import (
	"context"
	"sync"

	"trading-core/pkg/db"
)

// Manager keeps an in-memory view of positions (and later open orders) while persisting to DB for durability.
type Manager struct {
	mu        sync.RWMutex
	positions map[string]db.Position
	db        *db.Database
}

func NewManager(database *db.Database) *Manager {
	return &Manager{
		db:        database,
		positions: make(map[string]db.Position),
	}
}

// Load seeds in-memory state from DB on startup.
func (m *Manager) Load(ctx context.Context) error {
	if m.db == nil {
		return nil
	}
	pos, err := m.db.ListPositions(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range pos {
		m.positions[p.Symbol] = p
	}
	return nil
}

// Position returns the latest in-memory snapshot for a symbol.
func (m *Manager) Position(symbol string) db.Position {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.positions[symbol]
}

// Positions returns a snapshot of all positions.
func (m *Manager) Positions() []db.Position {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]db.Position, 0, len(m.positions))
	for _, p := range m.positions {
		res = append(res, p)
	}
	return res
}

// RecordFill adjusts position in-memory and persists it.
// This is a simplified PnL model; extend as needed when real fills are available.
func (m *Manager) RecordFill(ctx context.Context, symbol, side string, qty, price float64) (db.Position, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p := m.positions[symbol]
	oldQty := p.Qty
	oldAvg := p.AvgPrice

	var newQty float64
	var newAvg float64

	switch side {
	case "BUY":
		newQty = oldQty + qty
		if newQty != 0 {
			newAvg = (oldAvg*oldQty + price*qty) / newQty
		} else {
			newAvg = 0
		}
	case "SELL":
		newQty = oldQty - qty
		newAvg = oldAvg // keep average from existing; realized PnL tracking can be added later
	default:
		newQty = oldQty
		newAvg = oldAvg
	}

	p.Symbol = symbol
	p.Qty = newQty
	p.AvgPrice = newAvg

	if m.db != nil {
		_ = m.db.UpsertPosition(ctx, p)
	}
	m.positions[symbol] = p
	return p, nil
}

// SetPosition directly sets a position (used by reconciliation for syncing)
func (m *Manager) SetPosition(ctx context.Context, symbol string, qty, avgPrice float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p := db.Position{
		Symbol:   symbol,
		Qty:      qty,
		AvgPrice: avgPrice,
	}

	if m.db != nil {
		if err := m.db.UpsertPosition(ctx, p); err != nil {
			return err
		}
	}

	m.positions[symbol] = p
	return nil
}
