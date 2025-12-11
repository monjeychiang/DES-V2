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
	db              *sql.DB
	config          *RiskConfig
	metrics         *RiskMetrics
	strategyConfigs map[string]*StrategyRiskConfig // Per-strategy config cache
	mu              sync.RWMutex
}

// NewManager creates a new risk manager backed by the DB.
// If no active config exists it inserts DefaultConfig.
func NewManager(db *sql.DB) (*Manager, error) {
	mgr := &Manager{
		db:              db,
		metrics:         &RiskMetrics{},
		strategyConfigs: make(map[string]*StrategyRiskConfig),
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
		db:              nil,
		config:          &cfg,
		metrics:         &RiskMetrics{},
		strategyConfigs: make(map[string]*StrategyRiskConfig),
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

// QuickCheck performs fast pre-validation without full risk evaluation.
// Use this for immediate rejection of obviously blocked signals.
func (m *Manager) QuickCheck() QuickCheckResult {
	m.mu.RLock()
	cfg := *m.config
	metrics := *m.metrics
	m.mu.RUnlock()

	result := QuickCheckResult{
		Allowed:    true,
		LimitLevel: "NORMAL",
	}

	// Skip all checks if risk is disabled
	if !cfg.EnableRisk {
		return result
	}

	// Check daily trade limit
	if cfg.UseDailyTradeLimit && cfg.MaxDailyTrades > 0 {
		if metrics.DailyTrades >= cfg.MaxDailyTrades {
			result.Allowed = false
			result.Reason = "daily trade limit reached"
			result.LimitLevel = "LIMIT"
			result.UsageRatio = float64(metrics.DailyTrades) / float64(cfg.MaxDailyTrades)
			return result
		}
	}

	// Check daily loss limit with soft limits
	if cfg.UseDailyLossLimit && cfg.MaxDailyLoss > 0 {
		result.UsageRatio = metrics.DailyLosses / cfg.MaxDailyLoss
		result.LimitLevel = m.getLimitLevel(result.UsageRatio, cfg)

		if result.UsageRatio >= 1.0 {
			result.Allowed = false
			result.Reason = "daily loss limit reached"
			return result
		}
	}

	return result
}

// EvaluateFull performs complete risk evaluation with QuickCheck.
// This is the recommended single entry point for risk checks.
// Combines QuickCheck + EvaluateSignalWithStrategy in one call.
func (m *Manager) EvaluateFull(signal SignalInput, position Position, account Account, strategyID string) RiskDecision {
	// First do QuickCheck for fast rejection
	qr := m.QuickCheck()
	if !qr.Allowed {
		return RiskDecision{
			Allowed:    false,
			Reason:     qr.Reason,
			LimitLevel: qr.LimitLevel,
		}
	}

	// Then do full evaluation
	dec := m.EvaluateSignalWithStrategy(signal, position, account, strategyID)

	// Preserve limit level from QuickCheck if it was WARNING/CAUTION
	if qr.LimitLevel == "WARNING" || qr.LimitLevel == "CAUTION" {
		if dec.LimitLevel == "" || dec.LimitLevel == "NORMAL" {
			dec.LimitLevel = qr.LimitLevel
		}
		if dec.Warning == "" {
			dec.Warning = qr.Reason
		}
	}

	return dec
}

// GetStrategyConfig returns risk config for a specific strategy.
// Returns default config if not found.
func (m *Manager) GetStrategyConfig(strategyID string) StrategyRiskConfig {
	m.mu.RLock()
	if cfg, exists := m.strategyConfigs[strategyID]; exists && cfg != nil {
		m.mu.RUnlock()
		return *cfg
	}
	m.mu.RUnlock()

	// Try to load from DB
	if m.db != nil {
		cfg, err := m.loadStrategyConfigFromDB(strategyID)
		if err == nil {
			m.mu.Lock()
			m.strategyConfigs[strategyID] = &cfg
			m.mu.Unlock()
			return cfg
		}
	}

	// Return default
	return DefaultStrategyConfig(strategyID)
}

// loadStrategyConfigFromDB loads strategy config from database.
func (m *Manager) loadStrategyConfigFromDB(strategyID string) (StrategyRiskConfig, error) {
	cfg := StrategyRiskConfig{StrategyInstanceID: strategyID}
	var stopLoss, takeProfit sql.NullFloat64
	var useTrailing, enableRisk, usePosSize, useOrderSize int

	err := m.db.QueryRow(`
		SELECT max_position_size, min_order_size, max_order_size,
		       stop_loss, take_profit, use_trailing_stop, trailing_percent,
		       enable_risk, use_position_size_limit, use_order_size_limits, updated_at
		FROM strategy_risk_configs WHERE strategy_instance_id = ?
	`, strategyID).Scan(
		&cfg.MaxPositionSize, &cfg.MinOrderSize, &cfg.MaxOrderSize,
		&stopLoss, &takeProfit, &useTrailing, &cfg.TrailingPercent,
		&enableRisk, &usePosSize, &useOrderSize, &cfg.UpdatedAt,
	)
	if err != nil {
		return cfg, err
	}

	if stopLoss.Valid {
		cfg.StopLoss = &stopLoss.Float64
	}
	if takeProfit.Valid {
		cfg.TakeProfit = &takeProfit.Float64
	}
	cfg.UseTrailingStop = useTrailing == 1
	cfg.EnableRisk = enableRisk == 1
	cfg.UsePositionSizeLimit = usePosSize == 1
	cfg.UseOrderSizeLimits = useOrderSize == 1

	return cfg, nil
}

// SetStrategyConfig saves strategy-specific risk config.
func (m *Manager) SetStrategyConfig(cfg StrategyRiskConfig) error {
	m.mu.Lock()
	m.strategyConfigs[cfg.StrategyInstanceID] = &cfg
	m.mu.Unlock()

	if m.db == nil {
		return nil
	}

	var stopLoss, takeProfit interface{}
	if cfg.StopLoss != nil {
		stopLoss = *cfg.StopLoss
	}
	if cfg.TakeProfit != nil {
		takeProfit = *cfg.TakeProfit
	}

	_, err := m.db.Exec(`
		INSERT INTO strategy_risk_configs (
			strategy_instance_id, max_position_size, min_order_size, max_order_size,
			stop_loss, take_profit, use_trailing_stop, trailing_percent,
			enable_risk, use_position_size_limit, use_order_size_limits, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(strategy_instance_id) DO UPDATE SET
			max_position_size = excluded.max_position_size,
			min_order_size = excluded.min_order_size,
			max_order_size = excluded.max_order_size,
			stop_loss = excluded.stop_loss,
			take_profit = excluded.take_profit,
			use_trailing_stop = excluded.use_trailing_stop,
			trailing_percent = excluded.trailing_percent,
			enable_risk = excluded.enable_risk,
			use_position_size_limit = excluded.use_position_size_limit,
			use_order_size_limits = excluded.use_order_size_limits,
			updated_at = CURRENT_TIMESTAMP
	`,
		cfg.StrategyInstanceID, cfg.MaxPositionSize, cfg.MinOrderSize, cfg.MaxOrderSize,
		stopLoss, takeProfit, boolToInt(cfg.UseTrailingStop), cfg.TrailingPercent,
		boolToInt(cfg.EnableRisk), boolToInt(cfg.UsePositionSizeLimit), boolToInt(cfg.UseOrderSizeLimits),
	)
	return err
}

// EvaluateSignal evaluates a trading signal against risk rules.
func (m *Manager) EvaluateSignal(signal SignalInput, position Position, account Account) RiskDecision {
	m.mu.RLock()
	cfg := *m.config
	metrics := *m.metrics
	m.mu.RUnlock()

	dec := RiskDecision{Allowed: true}

	// 0. Global risk switch - bypass all checks if disabled
	if !cfg.EnableRisk {
		// Still set SL/TP from config if present
		dec.StopLoss = signal.Price * (1 - cfg.DefaultStopLoss)
		dec.TakeProfit = signal.Price * (1 + cfg.DefaultTakeProfit)
		dec.AdjustedSize = signal.Size
		return dec
	}

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
	if cfg.UseExposureLimit && cfg.MaxTotalExposure > 0 {
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

// EvaluateSignalWithStrategy evaluates signal using both global and strategy-level risk settings.
// This is the recommended method for layered risk control.
func (m *Manager) EvaluateSignalWithStrategy(signal SignalInput, position Position, account Account, strategyID string) RiskDecision {
	startTime := time.Now()

	m.mu.RLock()
	globalCfg := *m.config
	metrics := *m.metrics
	m.mu.RUnlock()

	// Get strategy-specific config
	strategyCfg := m.GetStrategyConfig(strategyID)

	// Defer metrics recording
	var dec RiskDecision
	defer func() {
		m.recordCheck(dec, time.Since(startTime).Nanoseconds())
	}()

	dec = RiskDecision{Allowed: true}

	// 0. Global risk switch - bypass ALL checks if disabled
	if !globalCfg.EnableRisk {
		return m.approveWithSLTP(signal, globalCfg, strategyCfg)
	}

	// 1. Strategy risk switch - bypass strategy checks if disabled
	if !strategyCfg.EnableRisk {
		return m.approveWithSLTP(signal, globalCfg, strategyCfg)
	}

	// ========== GLOBAL CHECKS (cannot be bypassed) ==========

	// G1. Daily trade count limit
	if globalCfg.UseDailyTradeLimit && globalCfg.MaxDailyTrades > 0 && metrics.DailyTrades >= globalCfg.MaxDailyTrades {
		dec.Allowed = false
		dec.Reason = fmt.Sprintf("daily trade limit reached: %d/%d", metrics.DailyTrades, globalCfg.MaxDailyTrades)
		return dec
	}

	// G2. Daily loss limit (with soft limits)
	if globalCfg.UseDailyLossLimit && globalCfg.MaxDailyLoss > 0 {
		usageRatio := metrics.DailyLosses / globalCfg.MaxDailyLoss
		dec.LimitLevel = m.getLimitLevel(usageRatio, globalCfg)

		if usageRatio >= 1.0 {
			// LIMIT: Hard stop
			dec.Allowed = false
			dec.Reason = fmt.Sprintf("daily loss limit exceeded: %.2f/%.2f", metrics.DailyLosses, globalCfg.MaxDailyLoss)
			return dec
		} else if usageRatio >= globalCfg.CautionThreshold {
			// CAUTION: Shrink order size
			dec.Warning = fmt.Sprintf("approaching daily loss limit (%.0f%%)", usageRatio*100)
			dec.AdjustedSize = signal.Size * globalCfg.CautionSizeRatio
			log.Printf("⚠️ Daily loss at %.0f%%, order shrunk: %.4f -> %.4f", usageRatio*100, signal.Size, dec.AdjustedSize)
		} else if usageRatio >= globalCfg.WarningThreshold {
			// WARNING: Allow but warn
			dec.Warning = fmt.Sprintf("approaching daily loss limit (%.0f%%)", usageRatio*100)
			log.Printf("⚠️ Daily loss at %.0f%% of limit", usageRatio*100)
		}
	}

	// G3. Total account exposure limit
	if globalCfg.UseExposureLimit && globalCfg.MaxTotalExposure > 0 {
		newNotional := signal.Size * signal.Price
		if account.TotalExposure+newNotional > globalCfg.MaxTotalExposure {
			dec.Allowed = false
			dec.Reason = "account total exposure limit reached"
			return dec
		}
	}

	// ========== STRATEGY CHECKS ==========

	orderValue := signal.Size * signal.Price
	dec.AdjustedSize = signal.Size

	// S1. Strategy position size limit
	if strategyCfg.UsePositionSizeLimit && strategyCfg.MaxPositionSize > 0 {
		if orderValue > strategyCfg.MaxPositionSize {
			dec.AdjustedSize = strategyCfg.MaxPositionSize / signal.Price
			log.Printf("[Strategy %s] Position size adjusted: %.4f -> %.4f", strategyID, signal.Size, dec.AdjustedSize)
		}

		// Check against existing position
		currentNotional := math.Abs(position.Quantity) * position.CurrentPrice
		newNotional := dec.AdjustedSize * signal.Price
		if currentNotional+newNotional > strategyCfg.MaxPositionSize {
			remaining := strategyCfg.MaxPositionSize - currentNotional
			if remaining <= 0 {
				dec.Allowed = false
				dec.Reason = fmt.Sprintf("[Strategy %s] position limit reached", strategyID)
				return dec
			}
			dec.AdjustedSize = remaining / signal.Price
		}
	}

	// S2. Strategy order size limits
	if strategyCfg.UseOrderSizeLimits {
		adjOrderValue := dec.AdjustedSize * signal.Price
		if adjOrderValue < strategyCfg.MinOrderSize {
			dec.Allowed = false
			dec.Reason = fmt.Sprintf("[Strategy %s] order too small: %.2f < %.2f", strategyID, adjOrderValue, strategyCfg.MinOrderSize)
			return dec
		}
		if strategyCfg.MaxOrderSize > 0 && adjOrderValue > strategyCfg.MaxOrderSize {
			dec.Allowed = false
			dec.Reason = fmt.Sprintf("[Strategy %s] order too large: %.2f > %.2f", strategyID, adjOrderValue, strategyCfg.MaxOrderSize)
			return dec
		}
	}

	// ========== CALCULATE SL/TP ==========
	dec = m.applySLTP(dec, signal, globalCfg, strategyCfg)

	log.Printf("[Strategy %s] Risk approved: %s %.4f @ %.2f, SL=%.2f, TP=%.2f",
		strategyID, signal.Action, dec.AdjustedSize, signal.Price, dec.StopLoss, dec.TakeProfit)

	return dec
}

// approveWithSLTP returns an approved decision with SL/TP calculated.
func (m *Manager) approveWithSLTP(signal SignalInput, globalCfg RiskConfig, strategyCfg StrategyRiskConfig) RiskDecision {
	dec := RiskDecision{
		Allowed:      true,
		AdjustedSize: signal.Size,
	}
	return m.applySLTP(dec, signal, globalCfg, strategyCfg)
}

// applySLTP applies stop loss and take profit to decision.
func (m *Manager) applySLTP(dec RiskDecision, signal SignalInput, globalCfg RiskConfig, strategyCfg StrategyRiskConfig) RiskDecision {
	// Use strategy SL/TP if set, otherwise use global defaults
	stopLoss := globalCfg.DefaultStopLoss
	takeProfit := globalCfg.DefaultTakeProfit

	if strategyCfg.StopLoss != nil {
		stopLoss = *strategyCfg.StopLoss
	}
	if strategyCfg.TakeProfit != nil {
		takeProfit = *strategyCfg.TakeProfit
	}

	if strings.EqualFold(signal.Action, "BUY") {
		dec.StopLoss = signal.Price * (1 - stopLoss)
		dec.TakeProfit = signal.Price * (1 + takeProfit)
	} else if strings.EqualFold(signal.Action, "SELL") {
		dec.StopLoss = signal.Price * (1 + stopLoss)
		dec.TakeProfit = signal.Price * (1 - takeProfit)
	}

	return dec
}

// getLimitLevel returns the limit level based on usage ratio.
func (m *Manager) getLimitLevel(usageRatio float64, cfg RiskConfig) string {
	if usageRatio >= 1.0 {
		return "LIMIT"
	} else if usageRatio >= cfg.CautionThreshold {
		return "CAUTION"
	} else if usageRatio >= cfg.WarningThreshold {
		return "WARNING"
	}
	return "NORMAL"
}

// recordCheck records metrics for a risk check.
func (m *Manager) recordCheck(dec RiskDecision, latencyNanos int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.ChecksTotal++
	m.metrics.CheckLatencyNanos += uint64(latencyNanos)
	m.metrics.CheckLatencyCount++

	if !dec.Allowed {
		m.metrics.RejectionsTotal++
	}
	if dec.Warning != "" {
		m.metrics.WarningsTotal++
	}
}

// GetRiskStats returns current risk monitoring statistics.
func (m *Manager) GetRiskStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	avgLatency := float64(0)
	if m.metrics.CheckLatencyCount > 0 {
		avgLatency = float64(m.metrics.CheckLatencyNanos) / float64(m.metrics.CheckLatencyCount) / 1e6 // to ms
	}

	return map[string]interface{}{
		"checks_total":     m.metrics.ChecksTotal,
		"rejections_total": m.metrics.RejectionsTotal,
		"warnings_total":   m.metrics.WarningsTotal,
		"avg_latency_ms":   avgLatency,
		"daily_trades":     m.metrics.DailyTrades,
		"daily_losses":     m.metrics.DailyLosses,
		"daily_pnl":        m.metrics.DailyPnL,
	}
}

// UpdateMetrics updates in-memory + DB risk metrics for a realized trade.
// trade.PnL should be net of fees.
func (m *Manager) UpdateMetrics(trade TradeResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// PnL is already net of fees, so avoid double-subtracting the fee here
	net := trade.PnL

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
