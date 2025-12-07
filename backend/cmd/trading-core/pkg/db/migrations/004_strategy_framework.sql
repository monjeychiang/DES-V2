-- Strategy Instances Table
-- Stores configuration for strategy instances
CREATE TABLE IF NOT EXISTS strategy_instances (
    id TEXT PRIMARY KEY,           -- UUID
    name TEXT NOT NULL,            -- User-friendly name (e.g., "BTC_MA_Cross")
    strategy_type TEXT NOT NULL,   -- Strategy implementation type (e.g., "ma_cross")
    symbol TEXT NOT NULL,          -- Trading pair
    interval TEXT NOT NULL,        -- Kline interval (e.g., "1h")
    parameters TEXT NOT NULL,      -- JSON configuration
    is_active BOOLEAN DEFAULT 1,   -- Enable/Disable switch
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Strategy States Table
-- Stores runtime state for persistence (decision variables only)
CREATE TABLE IF NOT EXISTS strategy_states (
    strategy_instance_id TEXT PRIMARY KEY,
    state_data TEXT NOT NULL,      -- JSON state data
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(strategy_instance_id) REFERENCES strategy_instances(id)
);

-- Add strategy_instance_id to existing tables
-- Note: SQLite doesn't support adding FK constraints via ALTER TABLE easily, 
-- so we just add the column for now.
ALTER TABLE orders ADD COLUMN strategy_instance_id TEXT;
-- Positions table might need a more complex migration if we want to enforce FK,
-- but for now we will handle the logic in code or add the column if it doesn't exist.
-- Since 'positions' is often a view or a simple table, let's check if we need to alter it.
-- Assuming 'positions' is the table managed by state/manager.go.
-- If positions are aggregated, we might need a separate 'strategy_positions' table.
-- For this phase, let's stick to the plan: "Add strategy_instance_id column to orders and positions tables"
-- However, 'positions' usually implies net position per symbol. 
-- If we want per-strategy positions, we might need a new table or change the PK of positions to (symbol, strategy_id).
-- Let's defer the 'positions' table change to Phase 3 when we tackle Order & Position Binding more deeply.
-- For now, just adding the column to orders is safe.
