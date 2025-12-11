package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"trading-core/internal/monitor"
	"trading-core/internal/order"
	"trading-core/pkg/db"
	exchange "trading-core/pkg/exchanges/common"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type createStrategyRequest struct {
	Name         string         `json:"name" binding:"required,min=1,max=120"`
	StrategyType string         `json:"strategy_type" binding:"required,min=1"`
	Symbol       string         `json:"symbol" binding:"required,min=1"`
	Interval     string         `json:"interval" binding:"required,min=1"`
	ConnectionID string         `json:"connection_id"`
	Parameters   map[string]any `json:"parameters"`
}

type listStrategiesQuery struct {
	Limit  int `form:"limit"`
	Offset int `form:"offset"`
}

type createOrderRequest struct {
	Symbol       string  `json:"symbol" binding:"required,min=1"`
	Side         string  `json:"side" binding:"required,oneof=BUY SELL"`
	Type         string  `json:"type" binding:"required,oneof=LIMIT MARKET"`
	Price        float64 `json:"price"`
	Qty          float64 `json:"qty" binding:"gt=0"`
	ConnectionID string  `json:"connection_id" binding:"required"`
}

type listOrdersQuery struct {
	Limit int `form:"limit"`
}

type createConnectionRequest struct {
	Name         string `json:"name" binding:"required,min=1"`
	ExchangeType string `json:"exchange_type" binding:"required,min=1"`
	APIKey       string `json:"api_key" binding:"required,min=1"`
	APISecret    string `json:"api_secret" binding:"required,min=1"`
}

type updateStrategyBindingRequest struct {
	ConnectionID string `json:"connection_id"`
}

func (q *listStrategiesQuery) normalize() {
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Limit > 200 {
		q.Limit = 200
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
}

func (q *listOrdersQuery) normalize() {
	if q.Limit <= 0 {
		q.Limit = 100
	}
	if q.Limit > 500 {
		q.Limit = 500
	}
}

func respondError(c *gin.Context, status int, code, msg string) {
	c.JSON(status, gin.H{
		"code":  code,
		"error": msg,
	})
}

func validateStrategyParams(strategyType string, params map[string]any) error {
	switch strings.ToLower(strategyType) {
	case "ma_cross":
		fast, ok := asFloat(params["fast"])
		slow, ok2 := asFloat(params["slow"])
		if !ok || !ok2 {
			return fmt.Errorf("ma_cross.fast and ma_cross.slow are required")
		}
		if fast <= 0 || slow <= 0 || fast >= slow {
			return fmt.Errorf("ma_cross.fast/slow must be >0 and fast < slow")
		}
		if size, ok := asFloat(params["size"]); ok && size <= 0 {
			return fmt.Errorf("ma_cross.size must be > 0")
		}
	case "rsi":
		period, ok := asFloat(params["period"])
		oversold, ok2 := asFloat(params["oversold"])
		overbought, ok3 := asFloat(params["overbought"])
		if !ok || !ok2 || !ok3 {
			return fmt.Errorf("rsi.period/oversold/overbought are required")
		}
		if period <= 0 {
			return fmt.Errorf("rsi.period must be > 0")
		}
		if oversold <= 0 || overbought <= 0 || oversold >= overbought {
			return fmt.Errorf("rsi oversold/overbought must be >0 and oversold < overbought")
		}
		if size, ok := asFloat(params["size"]); ok && size <= 0 {
			return fmt.Errorf("rsi.size must be > 0")
		}
	case "bollinger":
		period, ok := asFloat(params["period"])
		stddev, ok2 := asFloat(params["std_dev"])
		if !ok || !ok2 {
			return fmt.Errorf("bollinger.period and bollinger.std_dev are required")
		}
		if period <= 0 || stddev <= 0 {
			return fmt.Errorf("bollinger.period and bollinger.std_dev must be > 0")
		}
		if size, ok := asFloat(params["size"]); ok && size <= 0 {
			return fmt.Errorf("bollinger.size must be > 0")
		}
	default:
		// Unknown strategy type: no-op (could be validated elsewhere)
	}
	return nil
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

// createStrategy creates a new strategy instance bound to the current user (and optional connection).
func (s *Server) createStrategy(c *gin.Context) {
	userID := CurrentUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "user not authenticated")
		return
	}

	var req createStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request payload")
		return
	}
	if req.Parameters == nil {
		req.Parameters = map[string]any{}
	}

	if err := validateStrategyParams(req.StrategyType, req.Parameters); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_PARAMETERS", err.Error())
		return
	}

	ctx := c.Request.Context()
	// If connection_id provided, validate ownership and active status.
	if req.ConnectionID != "" {
		conn, err := s.DB.Queries().GetConnectionByID(ctx, userID, req.ConnectionID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				respondError(c, http.StatusBadRequest, "INVALID_CONNECTION", "invalid connection for current user")
			} else {
				respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
			}
			return
		}
		if !conn.IsActive {
			respondError(c, http.StatusBadRequest, "CONNECTION_INACTIVE", "connection is not active")
			return
		}
	}

	paramsJSON, err := json.Marshal(req.Parameters)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_PARAMETERS", "invalid parameters")
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
		respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
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

	var q listStrategiesQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_QUERY", "invalid query parameters")
		return
	}
	q.normalize()

	ctx := c.Request.Context()
	// Query strategies from DB, including optional binding info.
	rows, err := s.DB.DB.QueryContext(ctx, `
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
		ORDER BY si.created_at DESC
		LIMIT ? OFFSET ?
	`, userID, q.Limit, q.Offset)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
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
			// Record scan error for visibility
			_ = c.Error(err)
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

	if err := rows.Err(); err != nil {
		respondError(c, http.StatusInternalServerError, "DB_SCAN_ERROR", "failed to read strategies")
		return
	}

	c.Header("X-Result-Limit", strconv.Itoa(q.Limit))
	c.Header("X-Result-Offset", strconv.Itoa(q.Offset))
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
		respondError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "user not authenticated")
		return
	}

	var q listOrdersQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_QUERY", "invalid query parameters")
		return
	}
	q.normalize()

	orders, err := s.DB.Queries().GetOrdersByUser(c.Request.Context(), userID, q.Limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	c.Header("X-Result-Limit", strconv.Itoa(q.Limit))
	c.JSON(http.StatusOK, orders)
}

// getPositions returns current positions for the authenticated user.
func (s *Server) getPositions(c *gin.Context) {
	userID := CurrentUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "user not authenticated")
		return
	}

	positions, err := s.DB.Queries().GetPositionsByUser(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, positions)
}

// createOrder submits a manual order for the authenticated user on a specific connection.
func (s *Server) createOrder(c *gin.Context) {
	userID := CurrentUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "user not authenticated")
		return
	}
	if s.OrderQueue == nil {
		respondError(c, http.StatusServiceUnavailable, "QUEUE_UNAVAILABLE", "order queue not available")
		return
	}

	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request payload")
		return
	}
	if strings.EqualFold(req.Type, "LIMIT") && req.Price <= 0 {
		respondError(c, http.StatusBadRequest, "INVALID_PRICE", "price must be > 0 for LIMIT orders")
		return
	}

	ctx := c.Request.Context()
	conn, err := s.DB.Queries().GetConnectionByID(ctx, userID, req.ConnectionID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			respondError(c, http.StatusBadRequest, "INVALID_CONNECTION", "invalid connection for current user")
		} else {
			log.Printf("createOrder: failed to get connection %s for user %s: %v", req.ConnectionID, userID, err)
			respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		}
		return
	}
	if conn.APIKeyEncrypted != "" && s.KeyManager == nil {
		respondError(c, http.StatusInternalServerError, "CONFIG_ERROR", "encrypted connection requires KeyManager")
		return
	}
	if !conn.IsActive {
		respondError(c, http.StatusBadRequest, "CONNECTION_INACTIVE", "connection is not active")
		return
	}

	if s.UserBalances != nil {
		if mgr, err := s.UserBalances.GetOrCreate(userID); err == nil && mgr != nil {
			cost := req.Price * req.Qty
			if cost <= 0 {
				cost = req.Qty
			}
			if bal := mgr.GetBalance(); cost > bal.Available {
				respondError(c, http.StatusBadRequest, "INSUFFICIENT_BALANCE", "insufficient balance")
				return
			}
		}
	}

	var market string
	switch conn.ExchangeType {
	case "binance-spot":
		market = string(exchange.MarketSpot)
	case "binance-usdtfut":
		market = string(exchange.MarketUSDTFut)
	case "binance-coinfut":
		market = string(exchange.MarketCoinFut)
	default:
		respondError(c, http.StatusBadRequest, "UNSUPPORTED_EXCHANGE", "unsupported exchange type")
		return
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
		respondError(c, http.StatusServiceUnavailable, "ENGINE_UNAVAILABLE", err.Error())
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
		respondError(c, http.StatusServiceUnavailable, "ENGINE_UNAVAILABLE", err.Error())
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
			respondError(c, http.StatusBadRequest, "INVALID_FROM_DATE", "invalid from date")
			return
		}
	}
	if to != "" {
		toTime, err = time.Parse("2006-01-02", to)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_TO_DATE", "invalid to date")
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
		respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
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
		respondError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "unauthorized")
		return
	}

	conns, err := s.DB.ListConnectionsByUser(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
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
	defer func() {
		if r := recover(); r != nil {
			log.Printf("createConnection: panic: %v\n%s", r, debug.Stack())
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
	}()
	userID := CurrentUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "unauthorized")
		return
	}
	log.Printf("createConnection: user=%s", userID)

	var req createConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("createConnection: invalid payload: %v", err)
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request payload")
		return
	}
	log.Printf("createConnection: payload user=%s name=%s exch=%s", userID, req.Name, req.ExchangeType)

	if s.DB == nil || s.DB.DB == nil {
		log.Printf("createConnection: DB not initialized")
		respondError(c, http.StatusInternalServerError, "CONFIG_ERROR", "database not initialized")
		return
	}

	if s.KeyManager == nil {
		respondError(c, http.StatusInternalServerError, "CONFIG_ERROR", "KeyManager required for connection storage")
		return
	}

	now := time.Now()
	conn := db.Connection{
		ID:            uuid.NewString(),
		UserID:        userID,
		ExchangeType:  req.ExchangeType,
		Name:          req.Name,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
		LastRotatedAt: now,
	}

	// Always encrypt with KeyManager
	encKey, err := s.KeyManager.Encrypt(req.APIKey)
	if err != nil {
		log.Printf("createConnection: encrypt api_key failed: %v", err)
		respondError(c, http.StatusInternalServerError, "ENCRYPTION_ERROR", "failed to encrypt api_key")
		return
	}
	log.Printf("createConnection: encrypted api_key for user %s", userID)
	encSecret, err := s.KeyManager.Encrypt(req.APISecret)
	if err != nil {
		log.Printf("createConnection: encrypt api_secret failed: %v", err)
		respondError(c, http.StatusInternalServerError, "ENCRYPTION_ERROR", "failed to encrypt api_secret")
		return
	}
	conn.APIKeyEncrypted = encKey
	conn.APISecretEncrypted = encSecret
	conn.KeyVersion = s.KeyManager.CurrentVersion()
	// Keep plaintext fields for backward compatibility with components that still expect them.
	conn.APIKey = req.APIKey
	conn.APISecret = req.APISecret

	if err := s.DB.Queries().CreateConnectionEncrypted(c.Request.Context(), conn); err != nil {
		log.Printf("createConnection: db error for user %s: %v", userID, err)
		respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	log.Printf("createConnection: created id=%s user=%s exch=%s", conn.ID, userID, conn.ExchangeType)

	c.JSON(http.StatusCreated, gin.H{
		"id":            conn.ID,
		"name":          conn.Name,
		"exchange_type": conn.ExchangeType,
		"is_active":     conn.IsActive,
		"encrypted":     true,
		"key_version":   conn.KeyVersion,
		"last_rotated":  conn.LastRotatedAt,
		"created_at":    conn.CreatedAt,
		"updated_at":    conn.UpdatedAt,
	})
}

// deactivateConnection marks a connection as inactive (soft-delete) for the current user.
func (s *Server) deactivateConnection(c *gin.Context) {
	userID := CurrentUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "unauthorized")
		return
	}

	id := c.Param("id")
	if id == "" {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "missing connection id")
		return
	}

	if err := s.DB.DeactivateConnection(c.Request.Context(), id, userID); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			respondError(c, http.StatusForbidden, "FORBIDDEN", "connection does not belong to current user")
			return
		}
		respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deactivated"})
}

// updateStrategyBinding binds a strategy instance to a user + connection.
func (s *Server) updateStrategyBinding(c *gin.Context) {
	userID := CurrentUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "unauthorized")
		return
	}

	id := c.Param("id")
	if id == "" {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "missing strategy id")
		return
	}

	var req updateStrategyBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request payload")
		return
	}

	// Check ownership of strategy
	var owner sql.NullString
	err := s.DB.DB.QueryRow(`SELECT user_id FROM strategy_instances WHERE id = ?`, id).Scan(&owner)
	if err == sql.ErrNoRows {
		respondError(c, http.StatusNotFound, "STRATEGY_NOT_FOUND", "strategy not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if owner.Valid && owner.String != userID {
		respondError(c, http.StatusForbidden, "FORBIDDEN", "strategy does not belong to current user")
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
			respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		if count == 0 {
			respondError(c, http.StatusBadRequest, "INVALID_CONNECTION", "invalid connection for current user")
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
		respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
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
		respondError(c, http.StatusInternalServerError, "ENGINE_ERROR", err.Error())
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
		respondError(c, http.StatusInternalServerError, "ENGINE_ERROR", err.Error())
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
		respondError(c, http.StatusInternalServerError, "ENGINE_ERROR", err.Error())
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
		respondError(c, http.StatusInternalServerError, "ENGINE_ERROR", err.Error())
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
	if err := c.ShouldBindJSON(&params); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request payload")
		return
	}

	var strategyType string
	if err := s.DB.DB.QueryRow(`SELECT strategy_type FROM strategy_instances WHERE id = ?`, id).Scan(&strategyType); err != nil {
		if err == sql.ErrNoRows {
			respondError(c, http.StatusNotFound, "STRATEGY_NOT_FOUND", "strategy not found")
			return
		}
		respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if err := validateStrategyParams(strategyType, params); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_PARAMETERS", err.Error())
		return
	}

	if err := s.Engine.UpdateStrategyParams(c.Request.Context(), id, params); err != nil {
		respondError(c, http.StatusInternalServerError, "ENGINE_ERROR", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// canAccessStrategy checks if the current user can operate on the given strategy.
// It writes an error response and returns false if access is denied.
func (s *Server) canAccessStrategy(c *gin.Context, strategyID string) bool {
	userID := CurrentUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "UNAUTHENTICATED", "unauthorized")
		return false
	}

	var owner sql.NullString
	err := s.DB.DB.QueryRow(`SELECT user_id FROM strategy_instances WHERE id = ?`, strategyID).Scan(&owner)
	if err == sql.ErrNoRows {
		respondError(c, http.StatusNotFound, "STRATEGY_NOT_FOUND", "strategy not found")
		return false
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return false
	}

	// Allow if strategy is unowned (user_id NULL) or belongs to current user.
	if owner.Valid && owner.String != userID {
		respondError(c, http.StatusForbidden, "FORBIDDEN", "strategy does not belong to current user")
		return false
	}
	return true
}

// getMetrics returns system performance metrics.
func (s *Server) getMetrics(c *gin.Context) {
	if s.Metrics == nil {
		respondError(c, http.StatusServiceUnavailable, "METRICS_UNAVAILABLE", "metrics not available")
		return
	}
	snapshot := s.Metrics.GetSnapshot()
	c.JSON(http.StatusOK, snapshot)
}

// getPromMetrics returns a minimal Prometheus text exposition of key metrics.
func (s *Server) getPromMetrics(c *gin.Context) {
	if s.Metrics == nil {
		c.String(http.StatusServiceUnavailable, "# metrics not available\n")
		return
	}
	snapshot := s.Metrics.GetSnapshot()

	var b strings.Builder
	// Counters
	fmt.Fprintf(&b, "des_api_requests_total %d\n", snapshot.APIRequests)
	fmt.Fprintf(&b, "des_api_errors_total %d\n", snapshot.APIErrors)
	fmt.Fprintf(&b, "des_orders_processed_total %d\n", snapshot.OrdersProcessed)
	fmt.Fprintf(&b, "des_ticks_processed_total %d\n", snapshot.TicksProcessed)
	fmt.Fprintf(&b, "des_signals_generated_total %d\n", snapshot.SignalsGenerated)
	fmt.Fprintf(&b, "des_errors_total %d\n", snapshot.ErrorsCount)

	// Gauges for latency (ms)
	writeLatency := func(prefix string, ls monitor.LatencyStats) {
		if ls.Count == 0 {
			return
		}
		fmt.Fprintf(&b, "des_%s_latency_ms_avg %f\n", prefix, ls.Avg)
		fmt.Fprintf(&b, "des_%s_latency_ms_p50 %f\n", prefix, ls.P50)
		fmt.Fprintf(&b, "des_%s_latency_ms_p95 %f\n", prefix, ls.P95)
		fmt.Fprintf(&b, "des_%s_latency_ms_p99 %f\n", prefix, ls.P99)
	}
	writeLatency("api", snapshot.APILatency)
	writeLatency("order", snapshot.OrderLatency)
	writeLatency("order_gateway", snapshot.OrderGatewayLatency)
	writeLatency("order_persist", snapshot.OrderPersistLatency)
	writeLatency("strategy", snapshot.StrategyLatency)
	writeLatency("db", snapshot.DBLatency)

	// Gauges for system state
	fmt.Fprintf(&b, "des_gateway_total %d\n", snapshot.GatewayPool.TotalGateways)
	fmt.Fprintf(&b, "des_gateway_max %d\n", snapshot.GatewayPool.MaxSize)
	fmt.Fprintf(&b, "des_gateway_unhealthy %d\n", snapshot.GatewayPool.UnhealthyCount)
	for exType, count := range snapshot.GatewayPool.ByExchangeType {
		fmt.Fprintf(&b, "des_gateway_by_exchange{type=\"%s\"} %d\n", exType, count)
	}
	fmt.Fprintf(&b, "des_risk_active_users %d\n", snapshot.RiskActiveUsers)
	fmt.Fprintf(&b, "des_balance_active_users %d\n", snapshot.BalanceActiveUsers)
	fmt.Fprintf(&b, "des_goroutines %d\n", snapshot.GoroutineCount)
	fmt.Fprintf(&b, "des_heap_alloc_bytes %d\n", snapshot.HeapAlloc)
	fmt.Fprintf(&b, "des_heap_sys_bytes %d\n", snapshot.HeapSys)

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, b.String())
}

// getQueueMetrics returns order queue statistics.
func (s *Server) getQueueMetrics(c *gin.Context) {
	if s.OrderQueue == nil {
		respondError(c, http.StatusServiceUnavailable, "QUEUE_UNAVAILABLE", "order queue not available")
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
