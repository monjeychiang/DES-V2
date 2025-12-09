package risk

import (
	"fmt"
	"sync"
)

// StopLossManager manages stop loss orders and trailing stops
type StopLossManager struct {
	positions map[string]*StopLossPosition // key: strategyKey(strategyID, symbol)
	mu        sync.RWMutex
}

// StopLossPosition tracks stop loss for a position
type StopLossPosition struct {
	StrategyID     string // I4: Strategy ID for per-strategy tracking
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

// strategyKey creates a unique key for (strategyID, symbol) pair
func strategyKey(strategyID, symbol string) string {
	if strategyID == "" {
		return symbol // backward compatible
	}
	return strategyID + ":" + symbol
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

	key := strategyKey(pos.StrategyID, pos.Symbol)
	m.positions[key] = &pos
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

// ProtectionOrder represents a protective order (SL or TP).
type ProtectionOrder struct {
	Symbol      string
	Side        string  // BUY or SELL (opposite of position)
	Type        string  // STOP_MARKET or TAKE_PROFIT_MARKET
	StopPrice   float64 // Trigger price
	Qty         float64
	ReduceOnly  bool
	IsSL        bool   // true for stop-loss, false for take-profit
	LinkedOrder string // Original order ID
}

// GenerateProtectionOrders creates SL/TP orders for a filled position.
// Returns slice of orders to be enqueued (typically 0-2 orders).
func (m *StopLossManager) GenerateProtectionOrders(
	symbol string,
	positionSide string, // LONG or SHORT
	qty float64,
	linkedOrderID string,
) []ProtectionOrder {
	m.mu.RLock()
	pos, exists := m.positions[symbol]
	m.mu.RUnlock()

	if !exists || pos == nil {
		return nil
	}

	var orders []ProtectionOrder

	// Determine exit side (opposite of position)
	exitSide := "SELL"
	if positionSide == "SHORT" {
		exitSide = "BUY"
	}

	// Stop-Loss order
	if pos.StopLoss > 0 {
		orders = append(orders, ProtectionOrder{
			Symbol:      symbol,
			Side:        exitSide,
			Type:        "STOP_MARKET",
			StopPrice:   pos.StopLoss,
			Qty:         qty,
			ReduceOnly:  true,
			IsSL:        true,
			LinkedOrder: linkedOrderID,
		})
	}

	// Take-Profit order
	if pos.TakeProfit > 0 {
		orders = append(orders, ProtectionOrder{
			Symbol:      symbol,
			Side:        exitSide,
			Type:        "TAKE_PROFIT_MARKET",
			StopPrice:   pos.TakeProfit,
			Qty:         qty,
			ReduceOnly:  true,
			IsSL:        false,
			LinkedOrder: linkedOrderID,
		})
	}

	return orders
}

// GetAllPositions returns all tracked positions.
func (m *StopLossManager) GetAllPositions() map[string]StopLossPosition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]StopLossPosition, len(m.positions))
	for k, v := range m.positions {
		if v != nil {
			result[k] = *v
		}
	}
	return result
}
