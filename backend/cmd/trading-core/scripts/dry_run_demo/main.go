package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"

	"trading-core/internal/order"
	"trading-core/pkg/config"
)

// dry_run_demo simulates a few realistic order flows using the in‑memory
// MockExecutor. It does not touch the exchange or database.
//
// Usage (from backend/cmd/trading-core):
//   go run ./scripts/dry_run_demo
//
// It will:
//   1) BUY then SELL the same symbol within balance limits.
//   2) Try a BUY that exceeds balance to test risk of insufficient funds.
//   3) Print final mock positions and balance.

func main() {
	log.Println("=== DRY-RUN demo starting ===")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config error: %v", err)
	}

	initialBalance := cfg.DryRunInitialBalance
	if initialBalance <= 0 {
		initialBalance = 10000
	}

	ctx := context.Background()

	// We don't need a real executor or gateway here; MockExecutor will handle everything.
	dry := order.NewDryRunExecutor(order.ModeDryRun, nil, initialBalance)

	symbol := "BTCUSDT"

	log.Printf("[SCENARIO 1] Simple BUY then SELL on %s", symbol)
	buyOrder := order.Order{
		ID:        uuid.NewString(),
		Symbol:    symbol,
		Side:      "BUY",
		Type:      "LIMIT",
		Price:     100.0,
		Qty:       0.1,
		Status:    "NEW",
		CreatedAt: time.Now(),
	}
	dry.Execute(ctx, buyOrder)

	sellOrder := order.Order{
		ID:        uuid.NewString(),
		Symbol:    symbol,
		Side:      "SELL",
		Type:      "LIMIT",
		Price:     105.0,
		Qty:       0.1,
		Status:    "NEW",
		CreatedAt: time.Now(),
	}
	dry.Execute(ctx, sellOrder)

	log.Printf("[SCENARIO 2] Oversized BUY to trigger insufficient balance")
	bigBuy := order.Order{
		ID:        uuid.NewString(),
		Symbol:    symbol,
		Side:      "BUY",
		Type:      "LIMIT",
		Price:     100000.0,
		Qty:       1.0,
		Status:    "NEW",
		CreatedAt: time.Now(),
	}
	dry.Execute(ctx, bigBuy)

	log.Println("[SCENARIO DONE] Final DRY-RUN state:")
	dry.PrintState()

	log.Println("=== DRY-RUN demo finished ===")
}
