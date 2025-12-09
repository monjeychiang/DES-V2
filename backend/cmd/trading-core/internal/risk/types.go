package risk

import (
	"time"
)

// Failure mode constants
const (
	FailModeClose = "FAIL_CLOSE" // Default: reject on error
	FailModeLimit = "FAIL_LIMIT" // Use fallback size on error
)

// QuickCheckResult represents fast pre-validation result
type QuickCheckResult struct {
	Allowed    bool    `json:"allowed"`
	Reason     string  `json:"reason,omitempty"`
	LimitLevel string  `json:"limit_level"` // NORMAL/WARNING/CAUTION/LIMIT
	UsageRatio float64 `json:"usage_ratio"` // 0.0 - 1.0+
}

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
	EnableRisk           bool `json:"enable_risk"` // Global risk control switch
	UseDailyTradeLimit   bool `json:"use_daily_trade_limit"`
	UseDailyLossLimit    bool `json:"use_daily_loss_limit"`
	UseOrderSizeLimits   bool `json:"use_order_size_limits"`
	UsePositionSizeLimit bool `json:"use_position_size_limit"`
	UseExposureLimit     bool `json:"use_exposure_limit"` // Total exposure limit

	// Soft limit thresholds (P1 improvement)
	WarningThreshold float64 `json:"warning_threshold"`  // 0.8 = 80%
	CautionThreshold float64 `json:"caution_threshold"`  // 0.9 = 90%
	CautionSizeRatio float64 `json:"caution_size_ratio"` // 0.5 = shrink to 50%

	// Failure mode (P2 improvement)
	FailureMode  string  `json:"failure_mode"`  // FAIL_CLOSE, FAIL_LIMIT
	FallbackSize float64 `json:"fallback_size"` // Max order size when FAIL_LIMIT

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

	// Monitoring counters (P1 improvement)
	ChecksTotal       uint64 `json:"checks_total"`
	RejectionsTotal   uint64 `json:"rejections_total"`
	WarningsTotal     uint64 `json:"warnings_total"`
	CheckLatencyNanos uint64 `json:"check_latency_nanos"` // Sum of latencies
	CheckLatencyCount uint64 `json:"check_latency_count"` // Number of checks
}

// RiskDecision represents the result of risk evaluation
type RiskDecision struct {
	Allowed      bool    `json:"allowed"`
	Reason       string  `json:"reason"`
	Warning      string  `json:"warning,omitempty"` // Non-blocking warning
	LimitLevel   string  `json:"limit_level"`       // NORMAL/WARNING/CAUTION/LIMIT
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
		MaxDailyLoss:         2000.0,
		MaxDailyTrades:       20,
		MinOrderSize:         10.0,
		MaxOrderSize:         10000.0,
		MaxSlippage:          0.005,
		EnableRisk:           true,
		UseDailyTradeLimit:   true,
		UseDailyLossLimit:    true,
		UseOrderSizeLimits:   true,
		UsePositionSizeLimit: true,
		UseExposureLimit:     true,
		WarningThreshold:     0.8, // 80% - start warning
		CautionThreshold:     0.9, // 90% - shrink orders
		CautionSizeRatio:     0.5, // 50% - shrink to half
		FailureMode:          FailModeClose,
		FallbackSize:         100.0, // Fallback order size for FAIL_LIMIT mode
		IsActive:             true,
	}
}

// StrategyRiskConfig defines per-strategy risk settings
type StrategyRiskConfig struct {
	StrategyInstanceID string `json:"strategy_instance_id"`

	// Position & Order limits
	MaxPositionSize float64 `json:"max_position_size"`
	MinOrderSize    float64 `json:"min_order_size"`
	MaxOrderSize    float64 `json:"max_order_size"`

	// Stop Loss / Take Profit (nil means use global default)
	StopLoss        *float64 `json:"stop_loss"`
	TakeProfit      *float64 `json:"take_profit"`
	UseTrailingStop bool     `json:"use_trailing_stop"`
	TrailingPercent float64  `json:"trailing_percent"`

	// Enable switch
	EnableRisk bool `json:"enable_risk"`

	// Feature toggles
	UsePositionSizeLimit bool `json:"use_position_size_limit"`
	UseOrderSizeLimits   bool `json:"use_order_size_limits"`

	// Metadata
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultStrategyConfig returns default per-strategy risk config
func DefaultStrategyConfig(strategyID string) StrategyRiskConfig {
	return StrategyRiskConfig{
		StrategyInstanceID:   strategyID,
		MaxPositionSize:      1000.0,
		MinOrderSize:         10.0,
		MaxOrderSize:         10000.0,
		StopLoss:             nil, // Use global default
		TakeProfit:           nil, // Use global default
		UseTrailingStop:      false,
		TrailingPercent:      0.015,
		EnableRisk:           true,
		UsePositionSizeLimit: true,
		UseOrderSizeLimits:   true,
	}
}
