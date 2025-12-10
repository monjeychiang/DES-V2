package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"trading-core/internal/balance"
	"trading-core/internal/events"
	"trading-core/internal/order"
	"trading-core/internal/risk"
	"trading-core/internal/strategy"
	"trading-core/pkg/db"
)

// Impl implements the Service interface by composing existing modules.
type Impl struct {
	stratEngine *strategy.Engine
	riskMgr     *risk.Manager
	balanceMgr  *balance.Manager
	orderQueue  order.OrderQueue
	bus         *events.Bus
	db          *db.Database

	// Multi-user support (optional, for multi-user mode)
	multiUserRiskMgr *risk.MultiUserManager

	// System metadata
	meta SystemStatus
}

// Config holds the configuration for creating an engine implementation.
type Config struct {
	StratEngine *strategy.Engine
	RiskMgr     *risk.Manager
	BalanceMgr  *balance.Manager
	OrderQueue  order.OrderQueue
	Bus         *events.Bus
	DB          *db.Database
	Meta        SystemStatus

	// Multi-user support (optional)
	MultiUserRiskMgr *risk.MultiUserManager
}

// NewImpl creates a new engine implementation.
func NewImpl(cfg Config) *Impl {
	return &Impl{
		stratEngine:      cfg.StratEngine,
		riskMgr:          cfg.RiskMgr,
		balanceMgr:       cfg.BalanceMgr,
		orderQueue:       cfg.OrderQueue,
		bus:              cfg.Bus,
		db:               cfg.DB,
		meta:             cfg.Meta,
		multiUserRiskMgr: cfg.MultiUserRiskMgr,
	}
}

// --- Strategy Commands ---

func (e *Impl) StartStrategy(ctx context.Context, id string) error {
	if e.stratEngine == nil {
		return fmt.Errorf("strategy engine not available")
	}
	return e.stratEngine.ResumeStrategy(id)
}

func (e *Impl) PauseStrategy(ctx context.Context, id string) error {
	if e.stratEngine == nil {
		return fmt.Errorf("strategy engine not available")
	}
	return e.stratEngine.PauseStrategy(id)
}

func (e *Impl) StopStrategy(ctx context.Context, id string) error {
	if e.stratEngine == nil {
		return fmt.Errorf("strategy engine not available")
	}
	return e.stratEngine.StopStrategy(id)
}

func (e *Impl) PanicSellStrategy(ctx context.Context, id string, userID string) error {
	if e.stratEngine == nil {
		return fmt.Errorf("strategy engine not available")
	}

	// Get current position
	qty, err := e.stratEngine.GetStrategyPosition(id)
	if err != nil {
		return fmt.Errorf("failed to get position: %w", err)
	}

	if qty == 0 {
		return fmt.Errorf("no position to close")
	}

	// Determine side and submit close order
	side := "SELL"
	if qty < 0 {
		side = "BUY"
		qty = -qty
	}

	// Get strategy symbol
	var symbol string
	err = e.db.DB.QueryRowContext(ctx, `SELECT symbol FROM strategy_instances WHERE id = ?`, id).Scan(&symbol)
	if err != nil {
		return fmt.Errorf("failed to get strategy symbol: %w", err)
	}

	// Create panic order
	panicOrder := order.Order{
		ID:                 fmt.Sprintf("panic-%s-%d", id, time.Now().UnixMilli()),
		StrategyInstanceID: id,
		Symbol:             symbol,
		Side:               side,
		Type:               "MARKET",
		Qty:                qty,
	}

	// Enqueue the order
	if e.orderQueue != nil {
		e.orderQueue.Enqueue(panicOrder)
	}

	// Publish panic event
	if e.bus != nil {
		e.bus.Publish(events.EventStrategySignal, map[string]any{
			"strategy_id": id,
			"action":      "PANIC_SELL",
			"side":        side,
			"qty":         qty,
		})
	}

	return nil
}

func (e *Impl) UpdateStrategyParams(ctx context.Context, id string, params map[string]any) error {
	if e.stratEngine == nil {
		return fmt.Errorf("strategy engine not available")
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}

	return e.stratEngine.UpdateParams(id, paramsJSON)
}

func (e *Impl) BindStrategyConnection(ctx context.Context, strategyID, userID, connectionID string) error {
	_, err := e.db.DB.ExecContext(ctx, `
		UPDATE strategy_instances
		SET user_id = COALESCE(user_id, ?),
		    connection_id = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, userID, connectionID, strategyID)
	return err
}

// --- Strategy Queries ---

func (e *Impl) ListStrategies(ctx context.Context, userID string) ([]StrategyInfo, error) {
	rows, err := e.db.DB.QueryContext(ctx, `
		SELECT 
			si.id,
			si.name,
			si.strategy_type,
			si.symbol,
			si.interval,
			si.parameters,
			si.is_active,
			COALESCE(si.status, 'ACTIVE') as status,
			si.user_id,
			si.connection_id,
			c.name as connection_name,
			c.exchange_type,
			si.created_at,
			si.updated_at
		FROM strategy_instances si
		LEFT JOIN connections c ON si.connection_id = c.id
		WHERE si.user_id = ? OR si.user_id IS NULL
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var strategies []StrategyInfo
	for rows.Next() {
		var s StrategyInfo
		var paramsJSON string
		var userIDCol, connectionID, connectionName, connectionType sql.NullString

		if err := rows.Scan(
			&s.ID, &s.Name, &s.Type, &s.Symbol, &s.Interval,
			&paramsJSON, &s.IsActive, &s.Status,
			&userIDCol, &connectionID, &connectionName, &connectionType,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			continue
		}

		_ = json.Unmarshal([]byte(paramsJSON), &s.Parameters)
		s.UserID = nullableString(userIDCol)
		s.ConnectionID = nullableString(connectionID)
		s.ConnectionName = nullableString(connectionName)
		s.ConnectionExchangeType = nullableString(connectionType)

		strategies = append(strategies, s)
	}

	return strategies, nil
}

func (e *Impl) GetStrategyStatus(ctx context.Context, id string) (*StrategyStatus, error) {
	var status StrategyStatus
	status.ID = id

	// Get status from DB
	err := e.db.DB.QueryRowContext(ctx, `
		SELECT COALESCE(status, 'ACTIVE') FROM strategy_instances WHERE id = ?
	`, id).Scan(&status.Status)
	if err != nil {
		return nil, err
	}

	// Get position from engine
	if e.stratEngine != nil {
		pos, _ := e.stratEngine.GetStrategyPosition(id)
		status.Position = pos
	}

	return &status, nil
}

func (e *Impl) GetStrategyPosition(ctx context.Context, id string) (float64, error) {
	if e.stratEngine == nil {
		return 0, fmt.Errorf("strategy engine not available")
	}
	return e.stratEngine.GetStrategyPosition(id)
}

// --- Position & Order Queries ---

func (e *Impl) GetPositions(ctx context.Context) ([]Position, error) {
	dbPositions, err := e.db.ListPositions(ctx)
	if err != nil {
		return nil, err
	}

	positions := make([]Position, len(dbPositions))
	for i, p := range dbPositions {
		positions[i] = Position{
			Symbol:     p.Symbol,
			Quantity:   p.Qty,
			EntryPrice: p.AvgPrice,
			UpdatedAt:  p.UpdatedAt,
		}
	}
	return positions, nil
}

func (e *Impl) GetOpenOrders(ctx context.Context) ([]Order, error) {
	dbOrders, err := e.db.ListOpenOrders(ctx)
	if err != nil {
		return nil, err
	}

	orders := make([]Order, len(dbOrders))
	for i, o := range dbOrders {
		orders[i] = Order{
			ID:                 o.ID,
			StrategyInstanceID: o.StrategyInstanceID,
			Symbol:             o.Symbol,
			Side:               o.Side,
			Price:              o.Price,
			Qty:                o.Qty,
			Status:             o.Status,
			CreatedAt:          o.CreatedAt,
		}
	}
	return orders, nil
}

// --- Risk & Performance ---

func (e *Impl) GetRiskMetrics(ctx context.Context) (*RiskMetrics, error) {
	today := time.Now().Format("2006-01-02")
	var metrics RiskMetrics
	metrics.Date = today

	err := e.db.DB.QueryRowContext(ctx, `
		SELECT daily_pnl, daily_trades, daily_wins, daily_losses 
		FROM risk_metrics WHERE date = ?
	`, today).Scan(&metrics.DailyPnL, &metrics.DailyTrades, &metrics.DailyWins, &metrics.DailyLosses)

	if err == sql.ErrNoRows {
		return &metrics, nil // Return zeros
	}
	if err != nil {
		return nil, err
	}

	return &metrics, nil
}

func (e *Impl) GetStrategyPerformance(ctx context.Context, id string, from, to time.Time) (*Performance, error) {
	rows, err := e.db.DB.QueryContext(ctx, `
		SELECT 
			date(t.created_at) as d,
			SUM(
				CASE 
					WHEN UPPER(o.side) = 'SELL' THEN (t.price * t.qty)
					ELSE -(t.price * t.qty)
				END - t.fee
			) as pnl
		FROM trades t
		JOIN orders o ON t.order_id = o.id
		WHERE o.strategy_instance_id = ?
		  AND t.created_at >= ? AND t.created_at <= ?
		GROUP BY date(t.created_at)
		ORDER BY d ASC
	`, id, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	perf := &Performance{
		StrategyID: id,
		From:       from.Format("2006-01-02"),
		To:         to.Add(-24 * time.Hour).Format("2006-01-02"),
	}

	var equity float64
	for rows.Next() {
		var d string
		var pnl float64
		if err := rows.Scan(&d, &pnl); err != nil {
			continue
		}
		equity += pnl
		perf.Daily = append(perf.Daily, DailyPnL{Date: d, PnL: pnl, Equity: equity})
	}
	perf.TotalPnL = equity

	return perf, nil
}

// --- Balance ---

func (e *Impl) GetBalance(ctx context.Context) (*BalanceInfo, error) {
	if e.balanceMgr == nil {
		return nil, fmt.Errorf("balance manager not available")
	}

	bal := e.balanceMgr.GetBalance()
	return &BalanceInfo{
		Available: bal.Available,
		Locked:    bal.Locked,
		Total:     bal.Total,
	}, nil
}

// --- System ---

func (e *Impl) GetSystemStatus(ctx context.Context) *SystemStatus {
	status := e.meta
	status.ServerTime = time.Now().UTC()
	return &status
}

// --- Helpers ---

func nullableString(ns sql.NullString) *string {
	if ns.Valid {
		val := ns.String
		return &val
	}
	return nil
}
