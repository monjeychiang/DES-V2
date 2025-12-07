-- Risk Management System Database Schema

-- Risk configurations table
CREATE TABLE IF NOT EXISTS risk_configs (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    name                TEXT NOT NULL UNIQUE,
    
    -- Position Management
    max_position_size   REAL NOT NULL DEFAULT 1000.0,
    max_total_exposure  REAL NOT NULL DEFAULT 5000.0,
    default_leverage    REAL NOT NULL DEFAULT 1.0,
    
    -- Stop Loss / Take Profit
    default_stop_loss   REAL NOT NULL DEFAULT 0.02,    -- 2%
    default_take_profit REAL NOT NULL DEFAULT 0.05,    -- 5%
    use_trailing_stop   INTEGER NOT NULL DEFAULT 0,    -- boolean
    trailing_percent    REAL NOT NULL DEFAULT 0.015,   -- 1.5%
    
    -- Daily Limits
    max_daily_loss      REAL NOT NULL DEFAULT 500.0,
    max_daily_trades    INTEGER NOT NULL DEFAULT 20,
    
    -- Order Validation
    min_order_size      REAL NOT NULL DEFAULT 10.0,
    max_order_size      REAL NOT NULL DEFAULT 10000.0,
    max_slippage        REAL NOT NULL DEFAULT 0.005,   -- 0.5%
    
    -- Metadata
    is_active           INTEGER NOT NULL DEFAULT 1,
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Risk configuration change history
CREATE TABLE IF NOT EXISTS risk_config_history (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id   INTEGER NOT NULL,
    field_name  TEXT NOT NULL,
    old_value   TEXT,
    new_value   TEXT,
    changed_by  TEXT,
    changed_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (config_id) REFERENCES risk_configs(id)
);

-- Risk metrics tracking
CREATE TABLE IF NOT EXISTS risk_metrics (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    date                DATE NOT NULL,
    
    -- Daily Stats
    daily_pnl           REAL DEFAULT 0.0,
    daily_trades        INTEGER DEFAULT 0,
    daily_wins          INTEGER DEFAULT 0,
    daily_losses        INTEGER DEFAULT 0,
    
    -- Cumulative
    total_realized_pnl  REAL DEFAULT 0.0,
    max_drawdown        REAL DEFAULT 0.0,
    max_profit          REAL DEFAULT 0.0,
    
    -- Ratios
    win_rate            REAL DEFAULT 0.0,
    profit_factor       REAL DEFAULT 0.0,
    
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(date)
);

-- Insert default risk configuration
INSERT OR IGNORE INTO risk_configs (
    name, 
    max_position_size,
    max_total_exposure,
    default_leverage,
    default_stop_loss,
    default_take_profit,
    use_trailing_stop,
    trailing_percent,
    max_daily_loss,
    max_daily_trades,
    min_order_size,
    max_order_size,
    max_slippage,
    is_active
) VALUES (
    'default',
    1000.0,
    5000.0,
    1.0,
    0.02,
    0.05,
    0,
    0.015,
    500.0,
    20,
    10.0,
    10000.0,
    0.005,
    1
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_risk_configs_active ON risk_configs(is_active);
CREATE INDEX IF NOT EXISTS idx_risk_config_history_config ON risk_config_history(config_id);
CREATE INDEX IF NOT EXISTS idx_risk_metrics_date ON risk_metrics(date);
