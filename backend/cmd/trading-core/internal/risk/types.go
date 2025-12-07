package risk

import (
	"time"
)

// RiskConfig defines risk management parameters
type RiskConfig struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`

	// Position Management
	MaxPositionSize  float64 `json:"max_position_size"`
	MaxTotalExposure float64 `json:"max_total_exposure"`
	DefaultLeverage  float64 `json:"default_leverage"`

	// Stop Loss / Take Profit
	DefaultStopLoss   float64 `json:"default_stop_loss"`
	DefaultTakeProfit float64 `json:"default_take_profit"`
	UseTrailingStop   bool    `json:"use_trailing_stop"`
	TrailingPercent   float64 `json:"trailing_percent"`

	// Daily Limits
	MaxDailyLoss   float64 `json:"max_daily_loss"`
	MaxDailyTrades int     `json:"max_daily_trades"`

	// Order Validation
	MinOrderSize float64 `json:"min_order_size"`
	MaxOrderSize float64 `json:"max_order_size"`
	MaxSlippage  float64 `json:"max_slippage"`

	// Feature toggles
	UseDailyTradeLimit   bool `json:"use_daily_trade_limit"`
	UseDailyLossLimit    bool `json:"use_daily_loss_limit"`
	UseOrderSizeLimits   bool `json:"use_order_size_limits"`
	UsePositionSizeLimit bool `json:"use_position_size_limit"`

	// Metadata
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RiskMetrics tracks current risk status
type RiskMetrics struct {
	// Daily Statistics
	DailyPnL    float64 `json:"daily_pnl"`
	DailyTrades int     `json:"daily_trades"`
	DailyLosses float64 `json:"daily_losses"`

	// Cumulative
	TotalRealizedPnL float64 `json:"total_realized_pnl"`
	MaxDrawdown      float64 `json:"max_drawdown"`
	MaxProfit        float64 `json:"max_profit"`

	// Ratios
	WinRate      float64 `json:"win_rate"`
	ProfitFactor float64 `json:"profit_factor"`
}

// RiskDecision represents the result of risk evaluation
type RiskDecision struct {
	Allowed      bool    `json:"allowed"`
	Reason       string  `json:"reason"`
	AdjustedSize float64 `json:"adjusted_size"`
	StopLoss     float64 `json:"stop_loss"`
	TakeProfit   float64 `json:"take_profit"`
}

// Position represents a trading position
type Position struct {
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"` // LONG or SHORT
	EntryPrice    float64 `json:"entry_price"`
	CurrentPrice  float64 `json:"current_price"`
	Quantity      float64 `json:"quantity"`
	Value         float64 `json:"value"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
}

// Account represents account information
type Account struct {
	Balance          float64 `json:"balance"`
	AvailableBalance float64 `json:"available_balance"`
	LockedBalance    float64 `json:"locked_balance"`
	TotalExposure    float64 `json:"total_exposure"`
}

// DefaultConfig returns default risk configuration
func DefaultConfig() RiskConfig {
	return RiskConfig{
		Name:                 "default",
		MaxPositionSize:      1000.0,
		MaxTotalExposure:     5000.0,
		DefaultLeverage:      1.0,
		DefaultStopLoss:      0.02,
		DefaultTakeProfit:    0.05,
		UseTrailingStop:      false,
		TrailingPercent:      0.015,
		MaxDailyLoss:         500.0,
		MaxDailyTrades:       20,
		MinOrderSize:         10.0,
		MaxOrderSize:         10000.0,
		MaxSlippage:          0.005,
		UseDailyTradeLimit:   true,
		UseDailyLossLimit:    true,
		UseOrderSizeLimits:   true,
		UsePositionSizeLimit: true,
		IsActive:             true,
	}
}
