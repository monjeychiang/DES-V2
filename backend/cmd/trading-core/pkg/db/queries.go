// Package db provides user-isolated database queries for multi-tenant architecture.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrUserIDRequired = errors.New("user_id is required for data isolation")
	ErrNotFound       = errors.New("record not found")
)

// UserQueries provides user-isolated database queries.
type UserQueries struct {
	db *sql.DB
}

// NewUserQueries creates a new UserQueries instance.
func NewUserQueries(db *sql.DB) *UserQueries {
	return &UserQueries{db: db}
}

// ----------------------------------------
// Position Queries
// ----------------------------------------

// GetPositionsByUser returns all positions for a specific user.
func (q *UserQueries) GetPositionsByUser(ctx context.Context, userID string) ([]Position, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}

	rows, err := q.db.QueryContext(ctx, `
		SELECT symbol, qty, avg_price, COALESCE(user_id, ''), updated_at
		FROM user_positions
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query positions: %w", err)
	}
	defer rows.Close()

	var positions []Position
	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.Symbol, &p.Qty, &p.AvgPrice, &p.UserID, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan position: %w", err)
		}
		positions = append(positions, p)
	}
	return positions, rows.Err()
}

// UpsertPositionWithUser creates or updates a position for a user.
func (q *UserQueries) UpsertPositionWithUser(ctx context.Context, userID, symbol string, qty, avgPrice float64) error {
	if userID == "" {
		return ErrUserIDRequired
	}

	_, err := q.db.ExecContext(ctx, `
		INSERT INTO user_positions (symbol, qty, avg_price, user_id, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(symbol, user_id) DO UPDATE SET
			qty = excluded.qty,
			avg_price = excluded.avg_price,
			user_id = excluded.user_id,
			updated_at = CURRENT_TIMESTAMP
	`, symbol, qty, avgPrice, userID)

	return err
}

// ----------------------------------------
// Order Queries
// ----------------------------------------

// GetOrdersByUser returns orders for a specific user.
func (q *UserQueries) GetOrdersByUser(ctx context.Context, userID string, limit int) ([]Order, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}

	rows, err := q.db.QueryContext(ctx, `
		SELECT id, COALESCE(strategy_instance_id, ''), symbol, side, price, qty, 
		       COALESCE(filled_qty, 0), status, COALESCE(user_id, ''), created_at
		FROM orders
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query orders: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.StrategyInstanceID, &o.Symbol, &o.Side, &o.Price, &o.Qty, &o.FilledQty, &o.Status, &o.UserID, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// GetOpenOrdersByUser returns open orders for a specific user.
func (q *UserQueries) GetOpenOrdersByUser(ctx context.Context, userID string) ([]Order, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}

	rows, err := q.db.QueryContext(ctx, `
		SELECT id, COALESCE(strategy_instance_id, ''), symbol, side, price, qty, 
		       COALESCE(filled_qty, 0), status, COALESCE(user_id, ''), created_at
		FROM orders
		WHERE user_id = ? 
		  AND status IN ('NEW', 'PARTIALLY_FILLED')
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query open orders: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.StrategyInstanceID, &o.Symbol, &o.Side, &o.Price, &o.Qty, &o.FilledQty, &o.Status, &o.UserID, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// CreateOrderWithUser inserts a new order with user_id.
func (q *UserQueries) CreateOrderWithUser(ctx context.Context, o Order) error {
	if o.UserID == "" {
		return ErrUserIDRequired
	}

	_, err := q.db.ExecContext(ctx, `
		INSERT INTO orders (id, strategy_instance_id, symbol, side, price, qty, filled_qty, status, user_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP))
	`, o.ID, o.StrategyInstanceID, o.Symbol, o.Side, o.Price, o.Qty, o.FilledQty, o.Status, o.UserID, o.CreatedAt)

	return err
}

// ----------------------------------------
// Trade Queries
// ----------------------------------------

// GetTradesByUser returns trades for a specific user.
func (q *UserQueries) GetTradesByUser(ctx context.Context, userID string, limit int) ([]Trade, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}

	rows, err := q.db.QueryContext(ctx, `
		SELECT id, order_id, symbol, side, price, qty, COALESCE(fee, 0), COALESCE(user_id, ''), created_at
		FROM trades
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query trades: %w", err)
	}
	defer rows.Close()

	var trades []Trade
	for rows.Next() {
		var t Trade
		if err := rows.Scan(&t.ID, &t.OrderID, &t.Symbol, &t.Side, &t.Price, &t.Qty, &t.Fee, &t.UserID, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan trade: %w", err)
		}
		trades = append(trades, t)
	}
	return trades, rows.Err()
}

// CreateTradeWithUser inserts a new trade with user_id.
func (q *UserQueries) CreateTradeWithUser(ctx context.Context, t Trade) error {
	if t.UserID == "" {
		return ErrUserIDRequired
	}

	_, err := q.db.ExecContext(ctx, `
		INSERT INTO trades (id, order_id, symbol, side, price, qty, fee, user_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP))
	`, t.ID, t.OrderID, t.Symbol, t.Side, t.Price, t.Qty, t.Fee, t.UserID, t.CreatedAt)

	return err
}

// ----------------------------------------
// Connection Queries (with encryption support)
// ----------------------------------------

// GetConnectionsByUser returns all active connections for a user.
func (q *UserQueries) GetConnectionsByUser(ctx context.Context, userID string) ([]Connection, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}

	rows, err := q.db.QueryContext(ctx, `
		SELECT id, user_id, exchange_type, name, 
		       COALESCE(api_key, ''), COALESCE(api_secret, ''),
		       COALESCE(api_key_encrypted, ''), COALESCE(api_secret_encrypted, ''),
		       COALESCE(key_version, 1), is_active, created_at, updated_at
		FROM connections
		WHERE user_id = ? AND is_active = 1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query connections: %w", err)
	}
	defer rows.Close()

	var conns []Connection
	for rows.Next() {
		var c Connection
		if err := rows.Scan(&c.ID, &c.UserID, &c.ExchangeType, &c.Name,
			&c.APIKey, &c.APISecret, &c.APIKeyEncrypted, &c.APISecretEncrypted,
			&c.KeyVersion, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan connection: %w", err)
		}
		conns = append(conns, c)
	}
	return conns, rows.Err()
}

// GetConnectionByID returns a connection by ID, verifying user ownership.
func (q *UserQueries) GetConnectionByID(ctx context.Context, userID, connectionID string) (*Connection, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}

	var c Connection
	err := q.db.QueryRowContext(ctx, `
		SELECT id, user_id, exchange_type, name,
		       COALESCE(api_key, ''), COALESCE(api_secret, ''),
		       COALESCE(api_key_encrypted, ''), COALESCE(api_secret_encrypted, ''),
		       COALESCE(key_version, 1), is_active, created_at, updated_at
		FROM connections
		WHERE id = ? AND user_id = ?
	`, connectionID, userID).Scan(&c.ID, &c.UserID, &c.ExchangeType, &c.Name,
		&c.APIKey, &c.APISecret, &c.APIKeyEncrypted, &c.APISecretEncrypted,
		&c.KeyVersion, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query connection: %w", err)
	}
	return &c, nil
}

// CreateConnectionEncrypted creates a new connection with encrypted API keys.
func (q *UserQueries) CreateConnectionEncrypted(ctx context.Context, c Connection) error {
	if c.UserID == "" {
		return ErrUserIDRequired
	}

	_, err := q.db.ExecContext(ctx, `
		INSERT INTO connections (
			id, user_id, exchange_type, name,
			api_key, api_secret,
			api_key_encrypted, api_secret_encrypted,
			key_version, is_active, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, '', '', ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, c.ID, c.UserID, c.ExchangeType, c.Name, c.APIKeyEncrypted, c.APISecretEncrypted, c.KeyVersion)

	return err
}
