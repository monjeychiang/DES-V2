package db

import (
	"context"
	"database/sql"
	"math"
	"strings"
	"time"
)

// Order represents a trading order stored in the DB.
type Order struct {
	ID                 string
	StrategyInstanceID string
	Symbol             string
	Side               string
	Price              float64
	Qty                float64
	FilledQty          float64
	Status             string
	UserID             string // Multi-user isolation
	CreatedAt          time.Time
}

// Trade represents a fill stored in the DB.
type Trade struct {
	ID        string
	OrderID   string
	Symbol    string
	Side      string
	Price     float64
	Qty       float64
	Fee       float64
	UserID    string // Multi-user isolation
	CreatedAt time.Time
}

// Position tracks net position per symbol (global).
type Position struct {
	Symbol    string
	Qty       float64
	AvgPrice  float64
	UserID    string // Multi-user isolation
	UpdatedAt time.Time
}

// StrategyPosition tracks per-strategy exposure/PnL.
type StrategyPosition struct {
	StrategyInstanceID string
	Symbol             string
	Qty                float64
	AvgPrice           float64
	RealizedPnL        float64
	UpdatedAt          time.Time
}

// StrategyInstance represents a configured strategy row.
type StrategyInstance struct {
	ID           string
	Name         string
	StrategyType string
	Symbol       string
	Interval     string
	Parameters   string
	UserID       string
	ConnectionID string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// User represents an application user.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Connection represents a user's exchange connection/API key.
type Connection struct {
	ID                 string
	UserID             string
	ExchangeType       string
	Name               string
	APIKey             string
	APISecret          string
	APIKeyEncrypted    string // Phase 1: encrypted storage
	APISecretEncrypted string // Phase 1: encrypted storage
	KeyVersion         int    // Phase 1: key version
	IsActive           bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CreateOrder inserts a new order row.
func (d *Database) CreateOrder(ctx context.Context, o Order) error {
	_, err := d.DB.ExecContext(ctx, `
		INSERT INTO orders (
			id, strategy_instance_id, symbol, side, price, qty, filled_qty, status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP))
	`,
		o.ID, o.StrategyInstanceID, o.Symbol, o.Side, o.Price, o.Qty, o.FilledQty, o.Status, o.CreatedAt,
	)
	return err
}

// CreateTrade inserts a new trade row.
func (d *Database) CreateTrade(ctx context.Context, t Trade) error {
	_, err := d.DB.ExecContext(ctx, `
		INSERT INTO trades (
			id, order_id, symbol, side, price, qty, fee, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP))
	`,
		t.ID, t.OrderID, t.Symbol, t.Side, t.Price, t.Qty, t.Fee, t.CreatedAt,
	)
	return err
}

// UpdateOrderStatus sets the status of an order.
func (d *Database) UpdateOrderStatus(ctx context.Context, id, status string) error {
	_, err := d.DB.ExecContext(ctx, `UPDATE orders SET status = ? WHERE id = ?`, status, id)
	return err
}

// UpdateOrderFill sets status and filled quantity (and optionally price).
func (d *Database) UpdateOrderFill(ctx context.Context, id, status string, filledQty, price float64) error {
	_, err := d.DB.ExecContext(ctx, `
		UPDATE orders
		SET status = ?, filled_qty = ?, price = ?
		WHERE id = ?
	`, status, filledQty, price, id)
	return err
}

// UpsertPosition stores the latest position for a symbol.
func (d *Database) UpsertPosition(ctx context.Context, p Position) error {
	_, err := d.DB.ExecContext(ctx, `
		INSERT INTO positions (symbol, qty, avg_price, updated_at)
		VALUES (?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP))
		ON CONFLICT(symbol) DO UPDATE SET
			qty = excluded.qty,
			avg_price = excluded.avg_price,
			updated_at = COALESCE(excluded.updated_at, CURRENT_TIMESTAMP)
	`, p.Symbol, p.Qty, p.AvgPrice, p.UpdatedAt)
	return err
}

// ListOpenOrders returns orders that are not filled/closed.
func (d *Database) ListOpenOrders(ctx context.Context) ([]Order, error) {
	rows, err := d.DB.QueryContext(ctx, `
		SELECT id, strategy_instance_id, symbol, side, price, qty, filled_qty, status, created_at
		FROM orders WHERE status NOT IN ('FILLED','CANCELLED')
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.StrategyInstanceID, &o.Symbol, &o.Side, &o.Price, &o.Qty, &o.FilledQty, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, o)
	}
	return res, rows.Err()
}

// ListPositions returns all current positions.
func (d *Database) ListPositions(ctx context.Context) ([]Position, error) {
	rows, err := d.DB.QueryContext(ctx, `
		SELECT symbol, qty, avg_price, updated_at
		FROM positions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Position
	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.Symbol, &p.Qty, &p.AvgPrice, &p.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, p)
	}
	return res, rows.Err()
}

// UpdateStrategyPosition upserts per-strategy position and realized PnL.
// Simple logic: BUY increases qty/avg; SELL decreases qty and realizes PnL on the closed portion.
func (d *Database) UpdateStrategyPosition(ctx context.Context, strategyID, symbol, side string, qty, price float64) error {
	var sp StrategyPosition
	err := d.DB.QueryRowContext(ctx, `
		SELECT strategy_instance_id, symbol, qty, avg_price, realized_pnl, updated_at
		FROM strategy_positions WHERE strategy_instance_id = ?
	`, strategyID).Scan(&sp.StrategyInstanceID, &sp.Symbol, &sp.Qty, &sp.AvgPrice, &sp.RealizedPnL, &sp.UpdatedAt)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Initialize if not found
	if err == sql.ErrNoRows {
		sp = StrategyPosition{
			StrategyInstanceID: strategyID,
			Symbol:             symbol,
			Qty:                0,
			AvgPrice:           0,
			RealizedPnL:        0,
		}
	}

	switch strings.ToUpper(side) {
	case "BUY":
		newQty := sp.Qty + qty
		if math.Abs(newQty) < 1e-9 {
			// Position essentially closed, reset to avoid float precision issues
			sp.Qty = 0
			sp.AvgPrice = 0
		} else if newQty > 0 {
			sp.AvgPrice = (sp.AvgPrice*sp.Qty + price*qty) / newQty
			sp.Qty = newQty
		} else {
			sp.Qty = newQty
		}
	case "SELL":
		closeQty := math.Min(sp.Qty, qty)
		if closeQty > 0 {
			sp.RealizedPnL += (price - sp.AvgPrice) * closeQty
		}
		sp.Qty -= qty
		if sp.Qty < 1e-9 {
			sp.Qty = 0
			sp.AvgPrice = 0
		}
	default:
		// Unknown side, no-op
	}

	sp.Symbol = symbol
	sp.UpdatedAt = time.Now()

	_, execErr := d.DB.ExecContext(ctx, `
		INSERT INTO strategy_positions (strategy_instance_id, symbol, qty, avg_price, realized_pnl, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(strategy_instance_id) DO UPDATE SET
			symbol = excluded.symbol,
			qty = excluded.qty,
			avg_price = excluded.avg_price,
			realized_pnl = excluded.realized_pnl,
			updated_at = excluded.updated_at
	`, sp.StrategyInstanceID, sp.Symbol, sp.Qty, sp.AvgPrice, sp.RealizedPnL, sp.UpdatedAt)
	return execErr
}

// CreateUser inserts a new user row.
func (d *Database) CreateUser(ctx context.Context, u User) error {
	_, err := d.DB.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), COALESCE(?, CURRENT_TIMESTAMP))
	`, u.ID, strings.ToLower(u.Email), u.PasswordHash, u.CreatedAt, u.UpdatedAt)
	return err
}

// GetUserByEmail returns a user by email or nil if not found.
func (d *Database) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	row := d.DB.QueryRowContext(ctx, `
		SELECT id, email, password_hash, created_at, updated_at
		FROM users WHERE email = ?
	`, strings.ToLower(email))
	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// CreateConnection inserts a new exchange connection.
func (d *Database) CreateConnection(ctx context.Context, c Connection) error {
	_, err := d.DB.ExecContext(ctx, `
		INSERT INTO connections (
			id, user_id, exchange_type, name, api_key, api_secret, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), COALESCE(?, CURRENT_TIMESTAMP))
	`,
		c.ID, c.UserID, c.ExchangeType, c.Name, c.APIKey, c.APISecret, c.IsActive, c.CreatedAt, c.UpdatedAt,
	)
	return err
}

// ListConnectionsByUser returns all connections for a user.
func (d *Database) ListConnectionsByUser(ctx context.Context, userID string) ([]Connection, error) {
	rows, err := d.DB.QueryContext(ctx, `
		SELECT id, user_id, exchange_type, name, api_key, api_secret, is_active, created_at, updated_at
		FROM connections WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Connection
	for rows.Next() {
		var c Connection
		if err := rows.Scan(&c.ID, &c.UserID, &c.ExchangeType, &c.Name, &c.APIKey, &c.APISecret, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		res = append(res, c)
	}
	return res, rows.Err()
}

// DeactivateConnection marks a connection as inactive for a user.
func (d *Database) DeactivateConnection(ctx context.Context, id, userID string) error {
	_, err := d.DB.ExecContext(ctx, `
		UPDATE connections
		SET is_active = 0, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, id, userID)
	return err
}
