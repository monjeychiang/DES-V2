package strategy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents a strategy configuration entry in YAML.
type Config struct {
	ID         string                 `yaml:"id"`
	Name       string                 `yaml:"name"`
	Type       string                 `yaml:"type"`
	Symbol     string                 `yaml:"symbol"`
	Interval   string                 `yaml:"interval"`
	Parameters map[string]interface{} `yaml:"parameters"`
	IsActive   bool                   `yaml:"is_active"`
}

// ConfigFile represents the top-level YAML structure.
type ConfigFile struct {
	Strategies []Config `yaml:"strategies"`
}

// LoadConfig reads strategies from a YAML file.
func LoadConfig(path string) ([]Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var file ConfigFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, err
	}

	return file.Strategies, nil
}

// SyncConfigToDB upserts strategies from config into the database.
func SyncConfigToDB(db *sql.DB, configs []Config) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO strategy_instances (id, name, strategy_type, symbol, interval, parameters, is_active, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			strategy_type = excluded.strategy_type,
			symbol = excluded.symbol,
			interval = excluded.interval,
			parameters = excluded.parameters,
			is_active = excluded.is_active,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, cfg := range configs {
		paramsJSON, err := json.Marshal(cfg.Parameters)
		if err != nil {
			return fmt.Errorf("failed to marshal parameters for strategy %s: %w", cfg.Name, err)
		}

		_, err = stmt.Exec(
			cfg.ID,
			cfg.Name,
			cfg.Type,
			cfg.Symbol,
			cfg.Interval,
			string(paramsJSON),
			cfg.IsActive,
		)
		if err != nil {
			return fmt.Errorf("failed to upsert strategy %s: %w", cfg.Name, err)
		}
	}

	return tx.Commit()
}
