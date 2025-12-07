package risk

import (
	"fmt"
	"sync"
)

// StopLossManager manages stop loss orders and trailing stops
type StopLossManager struct {
	positions map[string]*StopLossPosition
	mu        sync.RWMutex
}

// StopLossPosition tracks stop loss for a position
type StopLossPosition struct {
	Symbol         string
	Side           string // LONG or SHORT
	EntryPrice     float64
	CurrentPrice   float64
	StopLoss       float64
	TakeProfit     float64
	TrailingStop   bool
	TrailingOffset float64 // Percentage offset
	HighWaterMark  float64 // For trailing stop
}

// NewStopLossManager creates a new stop loss manager
func NewStopLossManager() *StopLossManager {
	return &StopLossManager{
		positions: make(map[string]*StopLossPosition),
	}
}

// AddPosition adds a position to track
func (m *StopLossManager) AddPosition(pos StopLossPosition) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pos.Side == "LONG" {
		pos.HighWaterMark = pos.EntryPrice
	} else {
		pos.HighWaterMark = pos.EntryPrice
	}

	m.positions[pos.Symbol] = &pos
}

// UpdatePrice updates the current price and checks stop loss
func (m *StopLossManager) UpdatePrice(symbol string, price float64) *StopLossDecision {
	m.mu.Lock()
	defer m.mu.Unlock()

	pos, exists := m.positions[symbol]
	if !exists {
		return nil
	}

	pos.CurrentPrice = price

	// Update trailing stop
	if pos.TrailingStop {
		m.updateTrailingStop(pos)
	}

	// Check if stop loss triggered
	if m.isStopLossTriggered(pos) {
		return &StopLossDecision{
			Symbol:    symbol,
			Triggered: true,
			Reason:    fmt.Sprintf("Stop loss triggered at %.2f", price),
			Action:    "CLOSE",
			Price:     price,
		}
	}

	// Check if take profit triggered
	if m.isTakeProfitTriggered(pos) {
		return &StopLossDecision{
			Symbol:    symbol,
			Triggered: true,
			Reason:    fmt.Sprintf("Take profit triggered at %.2f", price),
			Action:    "CLOSE",
			Price:     price,
		}
	}

	return nil
}

// updateTrailingStop updates trailing stop level
func (m *StopLossManager) updateTrailingStop(pos *StopLossPosition) {
	if pos.Side == "LONG" {
		// For long position, track highest price
		if pos.CurrentPrice > pos.HighWaterMark {
			pos.HighWaterMark = pos.CurrentPrice
			// Update stop loss to trail
			pos.StopLoss = pos.HighWaterMark * (1 - pos.TrailingOffset)
		}
	} else {
		// For short position, track lowest price
		if pos.CurrentPrice < pos.HighWaterMark {
			pos.HighWaterMark = pos.CurrentPrice
			// Update stop loss to trail
			pos.StopLoss = pos.HighWaterMark * (1 + pos.TrailingOffset)
		}
	}
}

// isStopLossTriggered checks if stop loss is triggered
func (m *StopLossManager) isStopLossTriggered(pos *StopLossPosition) bool {
	if pos.Side == "LONG" {
		return pos.CurrentPrice <= pos.StopLoss
	}
	return pos.CurrentPrice >= pos.StopLoss
}

// isTakeProfitTriggered checks if take profit is triggered
func (m *StopLossManager) isTakeProfitTriggered(pos *StopLossPosition) bool {
	if pos.Side == "LONG" {
		return pos.CurrentPrice >= pos.TakeProfit
	}
	return pos.CurrentPrice <= pos.TakeProfit
}

// RemovePosition removes a position from tracking
func (m *StopLossManager) RemovePosition(symbol string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.positions, symbol)
}

// GetPosition gets a position
func (m *StopLossManager) GetPosition(symbol string) *StopLossPosition {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.positions[symbol]
}

// StopLossDecision represents stop loss decision
type StopLossDecision struct {
	Symbol    string
	Triggered bool
	Reason    string
	Action    string // CLOSE
	Price     float64
}
