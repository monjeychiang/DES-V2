package db

import (
	"database/sql"
	"fmt"
)

const schema = `
PRAGMA journal_mode=WAL;

CREATE TABLE IF NOT EXISTS strategies (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    params TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    strategy_instance_id TEXT,
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    price REAL NOT NULL,
    qty REAL NOT NULL,
    filled_qty REAL DEFAULT 0,
    status TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS trades (
    id TEXT PRIMARY KEY,
    order_id TEXT NOT NULL,
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    price REAL NOT NULL,
    qty REAL NOT NULL,
    fee REAL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS positions (
    symbol TEXT PRIMARY KEY,
    qty REAL NOT NULL,
    avg_price REAL NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS connections (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    exchange_type TEXT NOT NULL,
    name TEXT NOT NULL,
    api_key TEXT NOT NULL,
    api_secret TEXT NOT NULL,
    is_active BOOLEAN DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS risk_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    max_position_size REAL,
    max_total_exposure REAL,
    default_leverage REAL,
    default_stop_loss REAL,
    default_take_profit REAL,
    use_trailing_stop INTEGER,
    trailing_percent REAL,
    max_daily_loss REAL,
    max_daily_trades INTEGER,
    min_order_size REAL,
    max_order_size REAL,
    max_slippage REAL,
    use_daily_trade_limit INTEGER DEFAULT 1,
    use_daily_loss_limit INTEGER DEFAULT 1,
    use_order_size_limits INTEGER DEFAULT 1,
    use_position_size_limit INTEGER DEFAULT 1,
    is_active INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS risk_metrics (
    date TEXT PRIMARY KEY,
    daily_pnl REAL DEFAULT 0,
    daily_trades INTEGER DEFAULT 0,
    daily_wins INTEGER DEFAULT 0,
    daily_losses REAL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS strategy_instances (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    strategy_type TEXT NOT NULL,
    symbol TEXT NOT NULL,
    interval TEXT NOT NULL,
    parameters TEXT NOT NULL,
    user_id TEXT,
    connection_id TEXT,
    is_active BOOLEAN DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS strategy_states (
    strategy_instance_id TEXT PRIMARY KEY,
    state_data TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(strategy_instance_id) REFERENCES strategy_instances(id)
);

CREATE TABLE IF NOT EXISTS strategy_positions (
    strategy_instance_id TEXT PRIMARY KEY,
    symbol TEXT NOT NULL,
    qty REAL DEFAULT 0,
    avg_price REAL DEFAULT 0,
    realized_pnl REAL DEFAULT 0,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(strategy_instance_id) REFERENCES strategy_instances(id)
);

CREATE TABLE IF NOT EXISTS strategy_risk_configs (
    strategy_instance_id TEXT PRIMARY KEY,
    -- Position & Order limits
    max_position_size REAL,
    min_order_size REAL,
    max_order_size REAL,
    -- Stop Loss / Take Profit
    stop_loss REAL,
    take_profit REAL,
    use_trailing_stop INTEGER DEFAULT 0,
    trailing_percent REAL DEFAULT 0.015,
    -- Enable switch
    enable_risk INTEGER DEFAULT 1,
    -- Feature toggles
    use_position_size_limit INTEGER DEFAULT 1,
    use_order_size_limits INTEGER DEFAULT 1,
    -- Metadata
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(strategy_instance_id) REFERENCES strategy_instances(id)
);
`

// ApplyMigrations bootstraps the schema; keep lightweight for fast startup.
func ApplyMigrations(d *Database) error {
	if d == nil || d.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	if _, err := d.DB.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	// Lightweight, idempotent migrations for older DB files.
	if err := ensureColumn(d.DB, "orders", "filled_qty", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(d.DB, "orders", "strategy_instance_id", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(d.DB, "trades", "side", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	// Risk config feature toggles
	if err := ensureColumn(d.DB, "risk_configs", "use_daily_trade_limit", "INTEGER DEFAULT 1"); err != nil {
		return err
	}
	if err := ensureColumn(d.DB, "risk_configs", "use_daily_loss_limit", "INTEGER DEFAULT 1"); err != nil {
		return err
	}
	if err := ensureColumn(d.DB, "risk_configs", "use_order_size_limits", "INTEGER DEFAULT 1"); err != nil {
		return err
	}
	if err := ensureColumn(d.DB, "risk_configs", "use_position_size_limit", "INTEGER DEFAULT 1"); err != nil {
		return err
	}

	// Advanced Strategy Features
	if err := ensureColumn(d.DB, "strategy_instances", "status", "TEXT DEFAULT 'ACTIVE'"); err != nil {
		return err
	}
	if err := ensureColumn(d.DB, "strategy_instances", "user_id", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(d.DB, "strategy_instances", "connection_id", "TEXT"); err != nil {
		return err
	}

	// Create strategy_positions table if not exists
	if _, err := d.DB.Exec(`
		CREATE TABLE IF NOT EXISTS strategy_positions (
			strategy_instance_id TEXT PRIMARY KEY,
			symbol TEXT NOT NULL,
			qty REAL DEFAULT 0,
			avg_price REAL DEFAULT 0,
			realized_pnl REAL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(strategy_instance_id) REFERENCES strategy_instances(id)
		);
	`); err != nil {
		return fmt.Errorf("create strategy_positions table: %w", err)
	}

	// Phase 2 Features: Maker Only and Profit Target
	if err := ensureColumn(d.DB, "strategy_instances", "time_in_force", "TEXT DEFAULT 'GTC'"); err != nil {
		return err
	}
	if err := ensureColumn(d.DB, "strategy_instances", "profit_target", "REAL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(d.DB, "strategy_instances", "profit_target_type", "TEXT DEFAULT 'USDT'"); err != nil {
		return err
	}

	return nil
}

// ensureColumn adds a column if it does not already exist.
func ensureColumn(db *sql.DB, table, column, definition string) error {
	exists, err := columnExists(db, table, column)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	alter := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)
	if _, err := db.Exec(alter); err != nil {
		return fmt.Errorf("alter table %s add column %s: %w", table, column, err)
	}
	return nil
}

func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, fmt.Errorf("pragma table_info(%s): %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			colType    string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultVal, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
