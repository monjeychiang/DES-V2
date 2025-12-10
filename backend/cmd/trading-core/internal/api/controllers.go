package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"trading-core/internal/order"
	"trading-core/pkg/db"
	exchange "trading-core/pkg/exchanges/common"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// createStrategy creates a new strategy instance bound to the current user (and optional connection).
func (s *Server) createStrategy(c *gin.Context) {
	userID := CurrentUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	var req struct {
		Name         string         `json:"name"`
		StrategyType string         `json:"strategy_type"`
		Symbol       string         `json:"symbol"`
		Interval     string         `json:"interval"`
		ConnectionID string         `json:"connection_id"`
		Parameters   map[string]any `json:"parameters"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}
	if req.Name == "" || req.StrategyType == "" || req.Symbol == "" || req.Interval == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, strategy_type, symbol, interval are required"})
		return
	}

	// If connection_id provided, validate ownership and active status.
	if req.ConnectionID != "" {
		conn, err := s.DB.Queries().GetConnectionByID(c.Request.Context(), userID, req.ConnectionID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid connection for current user"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		if !conn.IsActive {
			c.JSON(http.StatusBadRequest, gin.H{"error": "connection is not active"})
			return
		}
	}

	paramsJSON, err := json.Marshal(req.Parameters)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parameters"})
		return
	}

	now := time.Now()
	id := uuid.NewString()
	_, err = s.DB.DB.Exec(`
		INSERT INTO strategy_instances (
			id, name, strategy_type, symbol, interval, parameters,
			user_id, connection_id, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)
	`, id, req.Name, req.StrategyType, req.Symbol, req.Interval, string(paramsJSON),
		userID, req.ConnectionID, now, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":            id,
		"name":          req.Name,
		"strategy_type": req.StrategyType,
		"symbol":        req.Symbol,
		"interval":      req.Interval,
		"parameters":    req.Parameters,
		"user_id":       userID,
		"connection_id": req.ConnectionID,
		"is_active":     false,
		"created_at":    now,
		"updated_at":    now,
	})
}

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

// getOrders returns recent orders for the authenticated user.
func (s *Server) getOrders(c *gin.Context) {
	userID := CurrentUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	orders, err := s.DB.Queries().GetOrdersByUser(c.Request.Context(), userID, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, orders)
}

// getPositions returns current positions for the authenticated user.
func (s *Server) getPositions(c *gin.Context) {
	userID := CurrentUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	positions, err := s.DB.Queries().GetPositionsByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, positions)
}

// createOrder submits a manual order for the authenticated user on a specific connection.
func (s *Server) createOrder(c *gin.Context) {
	userID := CurrentUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	if s.OrderQueue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "order queue not available"})
		return
	}

	var req struct {
		Symbol       string  `json:"symbol"`
		Side         string  `json:"side"`
		Type         string  `json:"type"`
		Price        float64 `json:"price"`
		Qty          float64 `json:"qty"`
		ConnectionID string  `json:"connection_id"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}
	if req.Symbol == "" || req.Side == "" || req.Type == "" || req.Qty <= 0 || req.ConnectionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol, side, type, qty, connection_id are required"})
		return
	}

	// Validate connection ownership and status.
	conn, err := s.DB.Queries().GetConnectionByID(c.Request.Context(), userID, req.ConnectionID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid connection for current user"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	// If connection uses encrypted keys but KeyManager is not configured, treat as server misconfiguration.
	if conn.APIKeyEncrypted != "" && s.KeyManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encrypted connection requires KeyManager"})
		return
	}
	if !conn.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connection is not active"})
		return
	}

	// Map exchange type to market.
	var market string
	switch conn.ExchangeType {
	case "binance-spot":
		market = string(exchange.MarketSpot)
	case "binance-usdtfut":
		market = string(exchange.MarketUSDTFut)
	case "binance-coinfut":
		market = string(exchange.MarketCoinFut)
	}

	o := order.Order{
		ID:           uuid.NewString(),
		Symbol:       req.Symbol,
		Side:         strings.ToUpper(req.Side),
		Type:         strings.ToUpper(req.Type),
		Price:        req.Price,
		Qty:          req.Qty,
		Status:       "NEW",
		CreatedAt:    time.Now(),
		Market:       market,
		UserID:       userID,
		ConnectionID: conn.ID,
	}

	s.OrderQueue.Enqueue(o)

	c.JSON(http.StatusAccepted, gin.H{
		"id":            o.ID,
		"symbol":        o.Symbol,
		"side":          o.Side,
		"type":          o.Type,
		"price":         o.Price,
		"qty":           o.Qty,
		"status":        o.Status,
		"connection_id": o.ConnectionID,
	})
}

// getBalance returns current balance information.
func (s *Server) getBalance(c *gin.Context) {
	// Prefer per-user balance when multi-user manager is available.
	userID := CurrentUserID(c)
	if userID != "" && s.UserBalances != nil {
		if mgr, err := s.UserBalances.GetOrCreate(userID); err == nil && mgr != nil {
			b := mgr.GetBalance()
			c.JSON(http.StatusOK, gin.H{
				"available": b.Available,
				"locked":    b.Locked,
				"total":     b.Total,
			})
			return
		}
	}

	// Fallback to global engine balance.
	bal, err := s.Engine.GetBalance(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
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
	metrics, err := s.Engine.GetRiskMetrics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, metrics)
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
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Use encryption if KeyManager is available
	if s.KeyManager != nil {
		encKey, err := s.KeyManager.Encrypt(req.APIKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt api_key"})
			return
		}
		encSecret, err := s.KeyManager.Encrypt(req.APISecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt api_secret"})
			return
		}
		conn.APIKeyEncrypted = encKey
		conn.APISecretEncrypted = encSecret
		conn.KeyVersion = s.KeyManager.CurrentVersion()

		if err := s.DB.Queries().CreateConnectionEncrypted(c.Request.Context(), conn); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		// Fallback to plaintext (legacy mode)
		conn.APIKey = req.APIKey
		conn.APISecret = req.APISecret

		if err := s.DB.CreateConnection(c.Request.Context(), conn); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":            conn.ID,
		"name":          conn.Name,
		"exchange_type": conn.ExchangeType,
		"is_active":     conn.IsActive,
		"encrypted":     s.KeyManager != nil,
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
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "connection does not belong to current user"})
			return
		}
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
	if err := s.Engine.StartStrategy(c.Request.Context(), id); err != nil {
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
	if err := s.Engine.PauseStrategy(c.Request.Context(), id); err != nil {
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
	if err := s.Engine.StopStrategy(c.Request.Context(), id); err != nil {
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

	userID := CurrentUserID(c)
	if err := s.Engine.PanicSellStrategy(c.Request.Context(), id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "panic_sell_triggered"})
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

	if err := s.Engine.UpdateStrategyParams(c.Request.Context(), id, params); err != nil {
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

	response := gin.H{
		"current_depth": s.OrderQueue.Len(),
	}

	// Try to get detailed metrics via type assertion
	if q, ok := s.OrderQueue.(*order.Queue); ok {
		metrics := q.GetMetrics()
		response["enqueued"] = metrics.Enqueued
		response["dequeued"] = metrics.Dequeued
		response["overflowed"] = metrics.Overflowed
		response["dropped"] = metrics.Dropped
		response["overflow_depth"] = q.OverflowLen()
		response["type"] = "in-memory"
	} else if pq, ok := s.OrderQueue.(*order.PersistentQueue); ok {
		metrics := pq.GetMetrics()
		response["written"] = metrics.Written
		response["recovered"] = metrics.Recovered
		response["completed"] = metrics.Completed
		response["failed"] = metrics.Failed
		response["type"] = "persistent"
	}

	c.JSON(http.StatusOK, response)
}
