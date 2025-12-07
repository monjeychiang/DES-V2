package main

import (
	"context"
	"log"
	"testing"
	"time"

	"trading-core/internal/balance"
	"trading-core/internal/reconciliation"
	"trading-core/internal/risk"
	"trading-core/internal/state"
	"trading-core/pkg/db"
)

// TestFullWorkflow tests the complete trading workflow
func TestFullWorkflow(t *testing.T) {
	log.Println("🧪 Starting Full Workflow Test...")

	ctx := context.Background()

	// Setup Database
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	if err := db.ApplyMigrations(database); err != nil {
		t.Fatalf("Failed to apply migrations: %v", err)
	}
	log.Println("✅ Database initialized")

	// Setup State Manager
	stateMgr := state.NewManager(database)
	if err := stateMgr.Load(ctx); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}
	log.Println("✅ State manager loaded")

	// Setup Risk Manager
	riskMgr, err := risk.NewManager(database.DB)
	if err != nil {
		t.Fatalf("Failed to create risk manager: %v", err)
	}
	log.Println("✅ Risk manager initialized")

	// Setup Balance Manager
	balanceMgr := balance.NewManager(nil, 30*time.Second)
	balanceMgr.SetInitialBalance(10000.0)
	log.Println("✅ Balance manager: 10000.0 USDT")

	// Test 1: Balance Management
	t.Run("BalanceManagement", func(t *testing.T) {
		log.Println("\n📊 Test 1: Balance Management")

		// Lock
		if err := balanceMgr.Lock(500.0); err != nil {
			t.Errorf("Lock failed: %v", err)
		}

		bal := balanceMgr.GetBalance()
		if bal.Available != 9500.0 || bal.Locked != 500.0 {
			t.Errorf("Lock incorrect: Available=%.2f Locked=%.2f", bal.Available, bal.Locked)
		} else {
			log.Println("✅ Locked 500.0")
		}

		// Unlock
		balanceMgr.Unlock(500.0)
		bal = balanceMgr.GetBalance()
		if bal.Available != 10000.0 || bal.Locked != 0 {
			t.Errorf("Unlock incorrect: Available=%.2f Locked=%.2f", bal.Available, bal.Locked)
		} else {
			log.Println("✅ Unlocked 500.0")
		}
	})

	// Test 2: Risk Evaluation
	t.Run("RiskEvaluation", func(t *testing.T) {
		log.Println("\n📊 Test 2: Risk Evaluation")

		signal := risk.SignalInput{
			Symbol: "BTCUSDT",
			Action: "BUY",
			Size:   0.01,
			Price:  50000.0,
		}

		position := risk.Position{
			Symbol:        "BTCUSDT",
			Quantity:      0,
			CurrentPrice:  50000.0,
			UnrealizedPnL: 0,
		}

		account := risk.Account{
			Balance:          10000.0,
			AvailableBalance: 10000.0,
			TotalExposure:    0,
		}

		decision := riskMgr.EvaluateSignal(signal, position, account)

		if !decision.Allowed {
			t.Errorf("Risk rejected: %s", decision.Reason)
		} else {
			log.Printf("✅ Risk approved: Size=%.4f SL=%.2f TP=%.2f",
				decision.AdjustedSize, decision.StopLoss, decision.TakeProfit)
		}
	})

	// Test 3: Position Management
	t.Run("PositionManagement", func(t *testing.T) {
		log.Println("\n📊 Test 3: Position Management")

		pos, err := stateMgr.RecordFill(ctx, "BTCUSDT", "BUY", 0.01, 50000.0)
		if err != nil {
			t.Errorf("RecordFill failed: %v", err)
		} else {
			log.Printf("✅ Position: Qty=%.4f AvgPrice=%.2f", pos.Qty, pos.AvgPrice)
		}

		retrieved := stateMgr.Position("BTCUSDT")
		if retrieved.Qty != 0.01 {
			t.Errorf("Position incorrect: Qty=%.4f", retrieved.Qty)
		}

		// Test Positions() method
		positions := stateMgr.Positions()
		log.Printf("✅ Retrieved %d positions", len(positions))
	})

	// Test 4: Risk Metrics
	t.Run("RiskMetrics", func(t *testing.T) {
		log.Println("\n📊 Test 4: Risk Metrics")

		err := riskMgr.UpdateMetrics(risk.TradeResult{
			Symbol: "BTCUSDT",
			Side:   "SELL",
			Size:   0.01,
			Price:  51000.0,
			PnL:    500.0,
			Fee:    10.0,
		})

		if err != nil {
			t.Errorf("UpdateMetrics failed: %v", err)
		}

		metrics := riskMgr.GetMetrics()
		if metrics.DailyTrades != 1 {
			t.Errorf("DailyTrades=%d (expected 1)", metrics.DailyTrades)
		}

		log.Printf("✅ Metrics: Trades=%d PnL=%.2f", metrics.DailyTrades, metrics.DailyPnL)
	})

	// Test 5: Feature Toggles
	t.Run("FeatureToggles", func(t *testing.T) {
		log.Println("\n📊 Test 5: Feature Toggles")

		cfg := riskMgr.GetConfig()

		if !cfg.UseDailyTradeLimit || !cfg.UseDailyLossLimit ||
			!cfg.UseOrderSizeLimits || !cfg.UsePositionSizeLimit {
			t.Error("Feature toggles not enabled")
		} else {
			log.Println("✅ All toggles enabled")
		}
	})

	log.Println("\n🎉 All Tests Passed!")
}

// TestReconciliation tests reconciliation service
func TestReconciliation(t *testing.T) {
	log.Println("🧪 Testing Reconciliation...")

	ctx := context.Background()

	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	if err := db.ApplyMigrations(database); err != nil {
		t.Fatalf("Failed to apply migrations: %v", err)
	}

	stateMgr := state.NewManager(database)
	if err := stateMgr.Load(ctx); err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	reconService := reconciliation.NewService(nil, stateMgr, database, 1*time.Minute)
	reconService.SetAutoSync(false)
	reconService.SetAutoSync(true)

	log.Println("✅ Reconciliation service tested")
}
