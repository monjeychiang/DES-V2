package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // SQLite driver
)

// Database wraps the SQL handle for easier swapping/testing.
type Database struct {
	DB *sql.DB
}

// New opens (and creates if needed) the SQLite database at path.
func New(path string) (*Database, error) {
	if path == "" {
		return nil, errors.New("database path is empty")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite prefers single writer.
	db.SetConnMaxLifetime(time.Hour)

	return &Database{DB: db}, nil
}

// Close releases the underlying DB handle.
func (d *Database) Close() error {
	if d == nil || d.DB == nil {
		return nil
	}
	return d.DB.Close()
}
