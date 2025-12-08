// Package engine provides a unified interface for the trading engine core.
// This package abstracts the internal trading logic from the API/Control layer,
// enabling future service separation (Phase 2 of Architecture Roadmap).
package engine

import (
	"context"
	"time"
)

// Service defines the interface for trading engine operations.
// The API layer should only interact with the engine through this interface.
type Service interface {
	// Strategy Commands
	StartStrategy(ctx context.Context, id string) error
	PauseStrategy(ctx context.Context, id string) error
	StopStrategy(ctx context.Context, id string) error
	PanicSellStrategy(ctx context.Context, id string, userID string) error
	UpdateStrategyParams(ctx context.Context, id string, params map[string]any) error
	BindStrategyConnection(ctx context.Context, strategyID, userID, connectionID string) error

	// Strategy Queries
	ListStrategies(ctx context.Context, userID string) ([]StrategyInfo, error)
	GetStrategyStatus(ctx context.Context, id string) (*StrategyStatus, error)
	GetStrategyPosition(ctx context.Context, id string) (float64, error)

	// Position & Order Queries
	GetPositions(ctx context.Context) ([]Position, error)
	GetOpenOrders(ctx context.Context) ([]Order, error)

	// Risk & Performance
	GetRiskMetrics(ctx context.Context) (*RiskMetrics, error)
	GetStrategyPerformance(ctx context.Context, id string, from, to time.Time) (*Performance, error)

	// Balance
	GetBalance(ctx context.Context) (*BalanceInfo, error)

	// System
	GetSystemStatus(ctx context.Context) *SystemStatus
}

// ReadOnlyDB defines read-only database operations for the Control/API layer.
// The API layer should use this for queries that don't affect trading state.
type ReadOnlyDB interface {
	// Connection queries
	ListConnectionsByUser(ctx context.Context, userID string) ([]Connection, error)

	// Order queries
	ListOpenOrders(ctx context.Context) ([]Order, error)
	GetOrderByID(ctx context.Context, id string) (*Order, error)

	// Position queries
	ListPositions(ctx context.Context) ([]Position, error)

	// User queries
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)

	// Raw query access (for custom reports)
	Query(ctx context.Context, query string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) Row
}

// Rows represents database query result rows
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
}

// Row represents a single database row
type Row interface {
	Scan(dest ...any) error
}
