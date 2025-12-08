package engine

import "time"

// StrategyInfo represents strategy information returned by the engine.
type StrategyInfo struct {
	ID                     string         `json:"id"`
	Name                   string         `json:"name"`
	Type                   string         `json:"type"`
	Symbol                 string         `json:"symbol"`
	Interval               string         `json:"interval"`
	Parameters             map[string]any `json:"parameters"`
	IsActive               bool           `json:"is_active"`
	Status                 string         `json:"status"`
	UserID                 *string        `json:"user_id,omitempty"`
	ConnectionID           *string        `json:"connection_id,omitempty"`
	ConnectionName         *string        `json:"connection_name,omitempty"`
	ConnectionExchangeType *string        `json:"connection_exchange_type,omitempty"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
}

// StrategyStatus represents the current status of a strategy.
type StrategyStatus struct {
	ID       string  `json:"id"`
	Status   string  `json:"status"`   // ACTIVE, PAUSED, STOPPED
	Position float64 `json:"position"` // Current position quantity
	PnL      float64 `json:"pnl"`      // Unrealized PnL
}

// Position represents a trading position.
type Position struct {
	ID                 string    `json:"id"`
	StrategyInstanceID string    `json:"strategy_instance_id"`
	Symbol             string    `json:"symbol"`
	Side               string    `json:"side"`
	Quantity           float64   `json:"quantity"`
	EntryPrice         float64   `json:"entry_price"`
	CurrentPrice       float64   `json:"current_price"`
	UnrealizedPnL      float64   `json:"unrealized_pnl"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Order represents a trading order.
type Order struct {
	ID                 string    `json:"id"`
	StrategyInstanceID string    `json:"strategy_instance_id,omitempty"`
	Symbol             string    `json:"symbol"`
	Side               string    `json:"side"`
	Type               string    `json:"type"`
	Price              float64   `json:"price"`
	Qty                float64   `json:"qty"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
}

// RiskMetrics represents current risk metrics.
type RiskMetrics struct {
	Date        string  `json:"date"`
	DailyPnL    float64 `json:"daily_pnl"`
	DailyTrades int     `json:"daily_trades"`
	DailyWins   int     `json:"daily_wins"`
	DailyLosses float64 `json:"daily_losses"`
}

// Performance represents strategy performance data.
type Performance struct {
	StrategyID string     `json:"strategy_id"`
	From       string     `json:"from"`
	To         string     `json:"to"`
	Daily      []DailyPnL `json:"daily"`
	TotalPnL   float64    `json:"total_pnl"`
}

// DailyPnL represents a single day's PnL.
type DailyPnL struct {
	Date   string  `json:"date"`
	PnL    float64 `json:"pnl"`
	Equity float64 `json:"equity"`
}

// BalanceInfo represents balance information.
type BalanceInfo struct {
	Available float64 `json:"available"`
	Locked    float64 `json:"locked"`
	Total     float64 `json:"total"`
}

// SystemStatus represents the system runtime status.
type SystemStatus struct {
	Mode        string    `json:"mode"`
	DryRun      bool      `json:"dry_run"`
	Venue       string    `json:"venue"`
	Symbols     []string  `json:"symbols"`
	UseMockFeed bool      `json:"use_mock_feed"`
	Version     string    `json:"version"`
	ServerTime  time.Time `json:"server_time"`
}

// Connection represents an exchange connection.
type Connection struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Name         string    `json:"name"`
	ExchangeType string    `json:"exchange_type"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// User represents a user account.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}
