package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"trading-core/internal/events"
	"trading-core/internal/strategy"
	"trading-core/pkg/db"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// getStrategies returns all configured strategies.
func (s *Server) getStrategies(c *gin.Context) {
	userID := CurrentUserID(c)

	// Query strategies from DB, including optional binding info.
	rows, err := s.DB.DB.Query(`
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var strategies []gin.H
	for rows.Next() {
		var (
			id, name, sType, symbol, interval, paramsJSON           string
			isActive                                                bool
			status                                                  string
			userIDCol, connectionID, connectionName, connectionType sql.NullString
			createdAt, updatedAt                                    time.Time
		)
		if err := rows.Scan(
			&id,
			&name,
			&sType,
			&symbol,
			&interval,
			&paramsJSON,
			&isActive,
			&status,
			&userIDCol,
			&connectionID,
			&connectionName,
			&connectionType,
			&createdAt,
			&updatedAt,
		); err != nil {
			continue
		}

		// Parse paramsJSON to object
		var params map[string]any
		_ = json.Unmarshal([]byte(paramsJSON), &params)

		strategies = append(strategies, gin.H{
			"id":                       id,
			"name":                     name,
			"type":                     sType,
			"symbol":                   symbol,
			"interval":                 interval,
			"parameters":               params,
			"is_active":                isActive,
			"status":                   status,
			"user_id":                  nullableString(userIDCol),
			"connection_id":            nullableString(connectionID),
			"connection_name":          nullableString(connectionName),
			"connection_exchange_type": nullableString(connectionType),
			"created_at":               createdAt,
			"updated_at":               updatedAt,
		})
	}
	c.JSON(http.StatusOK, strategies)
}

func nullableString(ns sql.NullString) *string {
	if ns.Valid {
		val := ns.String
		return &val
	}
	return nil
}

// getOrders returns recent orders.
func (s *Server) getOrders(c *gin.Context) {
	orders, err := s.DB.ListOpenOrders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, orders)
}

// getPositions returns current positions.
func (s *Server) getPositions(c *gin.Context) {
	positions, err := s.DB.ListPositions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, positions)
}

// getBalance returns current balance information.
func (s *Server) getBalance(c *gin.Context) {
	if s.BalanceMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "balance manager not available"})
		return
	}
	bal := s.BalanceMgr.GetBalance()
	c.JSON(http.StatusOK, bal)
}

// getSystemStatus exposes runtime mode/venue for the dashboard.
func (s *Server) getSystemStatus(c *gin.Context) {
	mode := "LIVE"
	if s.Meta.DryRun {
		mode = "DRY_RUN"
	}
	c.JSON(http.StatusOK, gin.H{
		"mode":          mode,
		"dry_run":       s.Meta.DryRun,
		"venue":         s.Meta.Venue,
		"symbols":       s.Meta.Symbols,
		"use_mock_feed": s.Meta.UseMockFeed,
		"version":       s.Meta.Version,
		"server_time":   time.Now().UTC(),
	})
}

// getRiskMetrics returns current risk metrics.
func (s *Server) getRiskMetrics(c *gin.Context) {
	if s.RiskMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "risk manager not available"})
		return
	}
	// Assuming RiskManager has a method to get metrics or we query DB
	// For now, let's query the risk_metrics table for today
	today := time.Now().Format("2006-01-02")
	var pnl, losses float64
	var trades, wins int

	err := s.DB.DB.QueryRow(`
		SELECT daily_pnl, daily_trades, daily_wins, daily_losses 
		FROM risk_metrics WHERE date = ?`, today).Scan(&pnl, &trades, &wins, &losses)

	if err != nil {
		// Return zeros if no row found
		c.JSON(http.StatusOK, gin.H{
			"date":         today,
			"daily_pnl":    0,
			"daily_trades": 0,
			"daily_wins":   0,
			"daily_losses": 0,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"date":         today,
		"daily_pnl":    pnl,
		"daily_trades": trades,
		"daily_wins":   wins,
		"daily_losses": losses,
	})
}

// getStrategyPerformance returns daily pnl and equity curve (cash-flow based) for a strategy.
// PnL is approximated as SELL notional minus BUY notional minus fee per trade.
func (s *Server) getStrategyPerformance(c *gin.Context) {
	id := c.Param("id")
	if !s.canAccessStrategy(c, id) {
		return
	}

	from := c.Query("from")
	to := c.Query("to")

	// Default range: last 30 days
	toTime := time.Now()
	fromTime := toTime.AddDate(0, 0, -30)
	var err error
	if from != "" {
		fromTime, err = time.Parse("2006-01-02", from)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from date"})
			return
		}
	}
	if to != "" {
		toTime, err = time.Parse("2006-01-02", to)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to date"})
			return
		}
		// include whole day
		toTime = toTime.Add(24 * time.Hour)
	}

	rows, err := s.DB.DB.Query(`
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
	`, id, fromTime, toTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type point struct {
		Date   string  `json:"date"`
		PNL    float64 `json:"pnl"`
		Equity float64 `json:"equity"`
	}
	var daily []point
	var equity float64
	for rows.Next() {
		var d string
		var pnl float64
		if err := rows.Scan(&d, &pnl); err != nil {
			continue
		}
		equity += pnl
		daily = append(daily, point{Date: d, PNL: pnl, Equity: equity})
	}

	c.JSON(http.StatusOK, gin.H{
		"strategy_id": id,
		"from":        fromTime.Format("2006-01-02"),
		"to":          toTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"daily":       daily,
		"total_pnl":   equity,
	})
}

// Exchange Connections (per-user)

// listConnections returns all connections for the current user.
func (s *Server) listConnections(c *gin.Context) {
	userID := CurrentUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	conns, err := s.DB.ListConnectionsByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var out []gin.H
	for _, conn := range conns {
		out = append(out, gin.H{
			"id":            conn.ID,
			"name":          conn.Name,
			"exchange_type": conn.ExchangeType,
			"is_active":     conn.IsActive,
			"created_at":    conn.CreatedAt,
			"updated_at":    conn.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

// createConnection creates a new exchange connection for the current user.
func (s *Server) createConnection(c *gin.Context) {
	userID := CurrentUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Name         string `json:"name"`
		ExchangeType string `json:"exchange_type"`
		APIKey       string `json:"api_key"`
		APISecret    string `json:"api_secret"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}
	if req.Name == "" || req.ExchangeType == "" || req.APIKey == "" || req.APISecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, exchange_type, api_key, api_secret are required"})
		return
	}

	now := time.Now()
	conn := db.Connection{
		ID:           uuid.NewString(),
		UserID:       userID,
		ExchangeType: req.ExchangeType,
		Name:         req.Name,
		APIKey:       req.APIKey,
		APISecret:    req.APISecret,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.DB.CreateConnection(c.Request.Context(), conn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":            conn.ID,
		"name":          conn.Name,
		"exchange_type": conn.ExchangeType,
		"is_active":     conn.IsActive,
		"created_at":    conn.CreatedAt,
		"updated_at":    conn.UpdatedAt,
	})
}

// deactivateConnection marks a connection as inactive (soft-delete) for the current user.
func (s *Server) deactivateConnection(c *gin.Context) {
	userID := CurrentUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing connection id"})
		return
	}

	if err := s.DB.DeactivateConnection(c.Request.Context(), id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deactivated"})
}

// updateStrategyBinding binds a strategy instance to a user + connection.
func (s *Server) updateStrategyBinding(c *gin.Context) {
	userID := CurrentUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing strategy id"})
		return
	}

	var req struct {
		ConnectionID string `json:"connection_id"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	// Check ownership of strategy
	var owner sql.NullString
	err := s.DB.DB.QueryRow(`SELECT user_id FROM strategy_instances WHERE id = ?`, id).Scan(&owner)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "strategy not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if owner.Valid && owner.String != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "strategy does not belong to current user"})
		return
	}

	// If a connection is specified, validate it belongs to the user and is active.
	if req.ConnectionID != "" {
		var count int
		err = s.DB.DB.QueryRow(`
			SELECT COUNT(1) FROM connections 
			WHERE id = ? AND user_id = ? AND is_active = 1
		`, req.ConnectionID, userID).Scan(&count)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if count == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid connection for current user"})
			return
		}
	}

	// Bind strategy to user + connection (user_id is set if empty).
	_, err = s.DB.DB.Exec(`
		UPDATE strategy_instances
		SET user_id = COALESCE(user_id, ?),
		    connection_id = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, userID, req.ConnectionID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "binding_updated"})
}

// Strategy Actions

func (s *Server) startStrategy(c *gin.Context) {
	id := c.Param("id")
	if !s.canAccessStrategy(c, id) {
		return
	}
	if err := s.StratEng.ResumeStrategy(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "started"})
}

func (s *Server) pauseStrategy(c *gin.Context) {
	id := c.Param("id")
	if !s.canAccessStrategy(c, id) {
		return
	}
	if err := s.StratEng.PauseStrategy(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

func (s *Server) stopStrategy(c *gin.Context) {
	id := c.Param("id")
	if !s.canAccessStrategy(c, id) {
		return
	}
	if err := s.StratEng.StopStrategy(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

func (s *Server) panicSellStrategy(c *gin.Context) {
	id := c.Param("id")
	if !s.canAccessStrategy(c, id) {
		return
	}

	// 1. Get Position
	qty, err := s.StratEng.GetStrategyPosition(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get position: " + err.Error()})
		return
	}

	if qty == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no position to close"})
		return
	}

	// 2. Determine Action
	action := "SELL"
	if qty < 0 {
		action = "BUY"
	}
	size := qty
	if size < 0 {
		size = -size
	}

	// 3. Emit Signal (Note: Panic Sell)
	// We need to know the symbol. Query DB or Engine?
	// Engine's GetStrategyPosition doesn't return symbol.
	// Let's query DB quickly or update GetStrategyPosition.
	// For speed, let's just query DB here.
	var symbol string
	err = s.DB.DB.QueryRow("SELECT symbol FROM strategy_instances WHERE id = ?", id).Scan(&symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get symbol: " + err.Error()})
		return
	}

	signal := strategy.Signal{
		StrategyID: id,
		Symbol:     symbol,
		Action:     action,
		Size:       size,
		Note:       "Panic Sell",
	}

	s.Bus.Publish(events.EventStrategySignal, signal)

	// 4. Stop Strategy
	if err := s.StratEng.StopStrategy(id); err != nil {
		// Log error but don't fail request since signal is sent
	}

	c.JSON(http.StatusOK, gin.H{"status": "panic_sell_triggered", "close_qty": size})
}

func (s *Server) updateStrategyParams(c *gin.Context) {
	id := c.Param("id")
	if !s.canAccessStrategy(c, id) {
		return
	}
	var params map[string]any
	if err := c.BindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jsonBytes, err := json.Marshal(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := s.StratEng.UpdateParams(id, jsonBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// canAccessStrategy checks if the current user can operate on the given strategy.
// It writes an error response and returns false if access is denied.
func (s *Server) canAccessStrategy(c *gin.Context, strategyID string) bool {
	userID := CurrentUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return false
	}

	var owner sql.NullString
	err := s.DB.DB.QueryRow(`SELECT user_id FROM strategy_instances WHERE id = ?`, strategyID).Scan(&owner)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "strategy not found"})
		return false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return false
	}

	// Allow if strategy is unowned (user_id NULL) or belongs to current user.
	if owner.Valid && owner.String != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "strategy does not belong to current user"})
		return false
	}
	return true
}

// getMetrics returns system performance metrics.
func (s *Server) getMetrics(c *gin.Context) {
	if s.Metrics == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "metrics not available"})
		return
	}
	snapshot := s.Metrics.GetSnapshot()
	c.JSON(http.StatusOK, snapshot)
}

// getQueueMetrics returns order queue statistics.
func (s *Server) getQueueMetrics(c *gin.Context) {
	if s.OrderQueue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "order queue not available"})
		return
	}
	metrics := s.OrderQueue.GetMetrics()
	c.JSON(http.StatusOK, gin.H{
		"enqueued":       metrics.Enqueued,
		"dequeued":       metrics.Dequeued,
		"overflowed":     metrics.Overflowed,
		"dropped":        metrics.Dropped,
		"current_depth":  s.OrderQueue.Len(),
		"overflow_depth": s.OrderQueue.OverflowLen(),
	})
}
