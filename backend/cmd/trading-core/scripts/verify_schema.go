package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := "test_btc.db" // Adjust path if needed
	fmt.Printf("Verifying database at: %s\n", dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	// 1. Verify strategy_positions table
	fmt.Println("\n1. Verifying strategy_positions table...")
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='strategy_positions'")
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	if rows.Next() {
		fmt.Println("✓ strategy_positions table exists")
	} else {
		fmt.Println("❌ strategy_positions table MISSING")
	}
	rows.Close()

	// 2. Verify status column in strategy_instances
	fmt.Println("\n2. Verifying status column in strategy_instances...")
	var sqlSchema string
	err = db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='strategy_instances'").Scan(&sqlSchema)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	if strings.Contains(sqlSchema, "status") {
		fmt.Println("✓ status column exists")
	} else {
		fmt.Println("❌ status column MISSING")
	}
}
