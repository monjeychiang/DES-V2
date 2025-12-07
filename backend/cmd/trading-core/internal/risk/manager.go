package risk

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"
)

// Manager handles risk configuration, evaluation, and metrics persistence.
type Manager struct {
	db      *sql.DB
	config  *RiskConfig
	metrics *RiskMetrics
	mu      sync.RWMutex
}

// NewManager creates a new risk manager backed by the DB.
// If no active config exists it inserts DefaultConfig.
func NewManager(db *sql.DB) (*Manager, error) {
	mgr := &Manager{
		db:      db,
		metrics: &RiskMetrics{},
	}

	if err := mgr.LoadConfig(); err != nil {
		if err == sql.ErrNoRows {
			def := DefaultConfig()
			if err := mgr.insertDefaultConfig(def); err != nil {
				return nil, fmt.Errorf("insert default risk config: %w", err)
			}
			mgr.config = &def
		} else {
			return nil, fmt.Errorf("load risk config: %w", err)
		}
	}

	cfg := mgr.GetConfig()
	log.Printf("Risk Manager initialized: stop_loss=%.1f%% take_profit=%.1f%%",
		cfg.DefaultStopLoss*100, cfg.DefaultTakeProfit*100)

	return mgr, nil
}

// NewInMemory creates a risk manager without DB persistence.
func NewInMemory(cfg RiskConfig) *Manager {
	return &Manager{
		db:      nil,
		config:  &cfg,
		metrics: &RiskMetrics{},
	}
}

// LoadConfig loads active risk configuration from DB or falls back to default.
func (m *Manager) LoadConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.db == nil {
		cfg := DefaultConfig()
		m.config = &cfg
		return nil
	}

	cfg := &RiskConfig{}
	query := `
		SELECT id, name, max_position_size, max_total_exposure, default_leverage,
		       default_stop_loss, default_take_profit, use_trailing_stop, trailing_percent,
		       max_daily_loss, max_daily_trades, min_order_size, max_order_size, max_slippage,
		       use_daily_trade_limit, use_daily_loss_limit, use_order_size_limits, use_position_size_limit,
		       is_active, created_at, updated_at
		FROM risk_configs
		WHERE is_active = 1
		LIMIT 1
	`

	var (
		useTrailing                                          int
		useDailyTrades, useDailyLoss, useOrderSize, usePosSz int
		isActive                                             int
	)

	err := m.db.QueryRow(query).Scan(
		&cfg.ID,
		&cfg.Name,
		&cfg.MaxPositionSize,
		&cfg.MaxTotalExposure,
		&cfg.DefaultLeverage,
		&cfg.DefaultStopLoss,
		&cfg.DefaultTakeProfit,
		&useTrailing,
		&cfg.TrailingPercent,
		&cfg.MaxDailyLoss,
		&cfg.MaxDailyTrades,
		&cfg.MinOrderSize,
		&cfg.MaxOrderSize,
		&cfg.MaxSlippage,
		&useDailyTrades,
		&useDailyLoss,
		&useOrderSize,
		&usePosSz,
		&isActive,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
	)
	if err != nil {
		return err
	}

	cfg.UseTrailingStop = useTrailing == 1
	cfg.UseDailyTradeLimit = useDailyTrades == 1
	cfg.UseDailyLossLimit = useDailyLoss == 1
	cfg.UseOrderSizeLimits = useOrderSize == 1
	cfg.UsePositionSizeLimit = usePosSz == 1
	cfg.IsActive = isActive == 1

	m.config = cfg
	return nil
}

func (m *Manager) insertDefaultConfig(cfg RiskConfig) error {
	if m.db == nil {
		m.config = &cfg
		return nil
	}
	_, err := m.db.Exec(`
		INSERT INTO risk_configs (
			name, max_position_size, max_total_exposure, default_leverage,
			default_stop_loss, default_take_profit, use_trailing_stop, trailing_percent,
			max_daily_loss, max_daily_trades, min_order_size, max_order_size, max_slippage,
			use_daily_trade_limit, use_daily_loss_limit, use_order_size_limits, use_position_size_limit,
			is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`,
		cfg.Name,
		cfg.MaxPositionSize,
		cfg.MaxTotalExposure,
		cfg.DefaultLeverage,
		cfg.DefaultStopLoss,
		cfg.DefaultTakeProfit,
		boolToInt(cfg.UseTrailingStop),
		cfg.TrailingPercent,
		cfg.MaxDailyLoss,
		cfg.MaxDailyTrades,
		cfg.MinOrderSize,
		cfg.MaxOrderSize,
		cfg.MaxSlippage,
		boolToInt(cfg.UseDailyTradeLimit),
		boolToInt(cfg.UseDailyLossLimit),
		boolToInt(cfg.UseOrderSizeLimits),
		boolToInt(cfg.UsePositionSizeLimit),
	)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// GetConfig returns a copy of current config.
func (m *Manager) GetConfig() RiskConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m.config
}

// UpdateConfig updates the active risk configuration row.
func (m *Manager) UpdateConfig(ctx context.Context, cfg RiskConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.db == nil {
		m.config = &cfg
		return nil
	}

	query := `
		UPDATE risk_configs
		SET max_position_size = ?, max_total_exposure = ?, default_leverage = ?,
		    default_stop_loss = ?, default_take_profit = ?, use_trailing_stop = ?,
		    trailing_percent = ?, max_daily_loss = ?, max_daily_trades = ?,
		    min_order_size = ?, max_order_size = ?, max_slippage = ?,
		    use_daily_trade_limit = ?, use_daily_loss_limit = ?,
		    use_order_size_limits = ?, use_position_size_limit = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND is_active = 1
	`

	useTrailing := boolToInt(cfg.UseTrailingStop)
	useDailyTrades := boolToInt(cfg.UseDailyTradeLimit)
	useDailyLoss := boolToInt(cfg.UseDailyLossLimit)
	useOrderSize := boolToInt(cfg.UseOrderSizeLimits)
	usePosSize := boolToInt(cfg.UsePositionSizeLimit)

	_, err := m.db.ExecContext(ctx, query,
		cfg.MaxPositionSize,
		cfg.MaxTotalExposure,
		cfg.DefaultLeverage,
		cfg.DefaultStopLoss,
		cfg.DefaultTakeProfit,
		useTrailing,
		cfg.TrailingPercent,
		cfg.MaxDailyLoss,
		cfg.MaxDailyTrades,
		cfg.MinOrderSize,
		cfg.MaxOrderSize,
		cfg.MaxSlippage,
		useDailyTrades,
		useDailyLoss,
		useOrderSize,
		usePosSize,
		m.config.ID,
	)
	if err != nil {
		return fmt.Errorf("update risk config: %w", err)
	}
	return m.LoadConfig()
}

// EvaluateSignal evaluates a trading signal against risk rules.
func (m *Manager) EvaluateSignal(signal SignalInput, position Position, account Account) RiskDecision {
	m.mu.RLock()
	cfg := *m.config
	metrics := *m.metrics
	m.mu.RUnlock()

	dec := RiskDecision{Allowed: true}

	// 1. Daily trade count limit.
	if cfg.UseDailyTradeLimit && cfg.MaxDailyTrades > 0 && metrics.DailyTrades >= cfg.MaxDailyTrades {
		dec.Allowed = false
		dec.Reason = fmt.Sprintf("daily trade limit reached: %d/%d", metrics.DailyTrades, cfg.MaxDailyTrades)
		return dec
	}

	// 2. Daily loss limit (uses realized PnL including fees).
	if cfg.UseDailyLossLimit && cfg.MaxDailyLoss > 0 && metrics.DailyLosses >= cfg.MaxDailyLoss {
		dec.Allowed = false
		dec.Reason = fmt.Sprintf("daily loss limit exceeded: %.2f/%.2f", metrics.DailyLosses, cfg.MaxDailyLoss)
		return dec
	}

	// 3. Basic order notional & min/max size.
	orderValue := signal.Size * signal.Price
	if cfg.MaxPositionSize > 0 && orderValue > cfg.MaxPositionSize {
		// Clip to max position size for this order.
		dec.AdjustedSize = cfg.MaxPositionSize / signal.Price
		log.Printf("Position size adjusted: %.4f -> %.4f", signal.Size, dec.AdjustedSize)
	} else {
		dec.AdjustedSize = signal.Size
	}

	if cfg.UseOrderSizeLimits {
		if orderValue < cfg.MinOrderSize {
			dec.Allowed = false
			dec.Reason = fmt.Sprintf("order size too small: %.2f < %.2f", orderValue, cfg.MinOrderSize)
			return dec
		}
		if cfg.MaxOrderSize > 0 && orderValue > cfg.MaxOrderSize {
			dec.Allowed = false
			dec.Reason = fmt.Sprintf("order size too large: %.2f > %.2f", orderValue, cfg.MaxOrderSize)
			return dec
		}
	}

	// 3b. Per-symbol exposure (existing position + new order).
	if cfg.MaxPositionSize > 0 {
		currentNotional := math.Abs(position.Quantity) * position.CurrentPrice
		newNotional := dec.AdjustedSize * signal.Price
		if currentNotional+newNotional > cfg.MaxPositionSize {
			remaining := cfg.MaxPositionSize - currentNotional
			if remaining <= 0 {
				dec.Allowed = false
				dec.Reason = "symbol exposure limit reached"
				return dec
			}
			newSize := remaining / signal.Price
			log.Printf("Symbol exposure adjusted: %.4f -> %.4f (limit %.2f)", dec.AdjustedSize, newSize, cfg.MaxPositionSize)
			dec.AdjustedSize = newSize
		}
	}

	// 3c. Total account exposure limit.
	if cfg.MaxTotalExposure > 0 {
		newNotional := dec.AdjustedSize * signal.Price
		if account.TotalExposure+newNotional > cfg.MaxTotalExposure {
			dec.Allowed = false
			dec.Reason = "account total exposure limit reached"
			return dec
		}
	}

	// 4. Calculate stop loss and take profit.
	if strings.EqualFold(signal.Action, "BUY") {
		dec.StopLoss = signal.Price * (1 - cfg.DefaultStopLoss)
		dec.TakeProfit = signal.Price * (1 + cfg.DefaultTakeProfit)
	} else if strings.EqualFold(signal.Action, "SELL") {
		dec.StopLoss = signal.Price * (1 + cfg.DefaultStopLoss)
		dec.TakeProfit = signal.Price * (1 - cfg.DefaultTakeProfit)
	}

	log.Printf("Risk approved: %s %.4f @ %.2f, SL=%.2f, TP=%.2f",
		signal.Action, dec.AdjustedSize, signal.Price, dec.StopLoss, dec.TakeProfit)

	return dec
}

// UpdateMetrics updates in-memory + DB risk metrics for a realized trade.
// trade.PnL should be net of fees.
func (m *Manager) UpdateMetrics(trade TradeResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Use net PnL = gross PnL - fee
	net := trade.PnL - trade.Fee

	m.metrics.DailyTrades++
	m.metrics.DailyPnL += net
	if net < 0 {
		m.metrics.DailyLosses += -net
	}

	m.metrics.TotalRealizedPnL += net
	if m.metrics.TotalRealizedPnL > m.metrics.MaxProfit {
		m.metrics.MaxProfit = m.metrics.TotalRealizedPnL
	}
	drawdown := m.metrics.MaxProfit - m.metrics.TotalRealizedPnL
	if drawdown > m.metrics.MaxDrawdown {
		m.metrics.MaxDrawdown = drawdown
	}

	if m.db == nil {
		return nil
	}

	// Persist aggregated daily metrics.
	today := time.Now().Format("2006-01-02")
	query := `
		INSERT INTO risk_metrics (date, daily_pnl, daily_trades, daily_wins, daily_losses)
		VALUES (?, ?, 1, ?, ?)
		ON CONFLICT(date) DO UPDATE SET
			daily_pnl = daily_pnl + ?,
			daily_trades = daily_trades + 1,
			daily_wins = daily_wins + ?,
			daily_losses = daily_losses + ?
	`

	wins := 0
	losses := 0.0
	if net > 0 {
		wins = 1
	} else if net < 0 {
		losses = -net
	}

	_, err := m.db.Exec(query,
		today, net, wins, losses,
		net, wins, losses,
	)
	return err
}

// ResetDailyMetrics resets in-memory daily counters (should be called at new day).
func (m *Manager) ResetDailyMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("Daily metrics reset. Prev: PnL=%.2f Trades=%d Losses=%.2f",
		m.metrics.DailyPnL, m.metrics.DailyTrades, m.metrics.DailyLosses)

	m.metrics.DailyPnL = 0
	m.metrics.DailyTrades = 0
	m.metrics.DailyLosses = 0
}

// GetMetrics returns current metrics snapshot.
func (m *Manager) GetMetrics() RiskMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m.metrics
}

// SignalInput represents a trading signal from strategy.
type SignalInput struct {
	Symbol string
	Action string // BUY, SELL
	Size   float64
	Price  float64
}

// TradeResult represents an executed trade result.
type TradeResult struct {
	Symbol string
	Side   string
	Size   float64
	Price  float64
	PnL    float64 // net of fees
	Fee    float64
}
