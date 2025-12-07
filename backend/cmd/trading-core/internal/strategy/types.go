package strategy

import (
	"encoding/json"
	"trading-core/internal/indicators"
)

// Signal is a decision emitted by a strategy.
type Signal struct {
	StrategyID string // The ID of the strategy instance
	Action     string // BUY, SELL, HOLD
	Symbol     string
	Size       float64
	Note       string
}

// Strategy defines the interface for all strategies.
type Strategy interface {
	// ID returns the unique instance ID
	ID() string
	// Name returns the human-readable name
	Name() string
	// OnTick processes a new price update
	OnTick(symbol string, price float64, ind map[string]float64) (*Signal, error)

	// State Management
	// GetState returns the serializable state of the strategy
	GetState() (json.RawMessage, error)
	// SetState restores the state of the strategy
	SetState(data json.RawMessage) error
}

// Context bundles shared services for strategies.
type Context struct {
	Indicators *indicators.Engine
}
