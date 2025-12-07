package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"trading-core/internal/events"
	"trading-core/internal/order"
	"trading-core/pkg/config"
	"trading-core/pkg/db"
	exfutcoin "trading-core/pkg/exchanges/binance/futures_coin"
	exfutusdt "trading-core/pkg/exchanges/binance/futures_usdt"
	exspot "trading-core/pkg/exchanges/binance/spot"
)

// This script tests Spot / USDT-M / COIN-M user data streams end‑to‑end:
// - creates DB + event bus
// - starts user streams (based on env/config)
// - logs every order/trade event seen by the handlers
//
// Usage (from backend/cmd/trading-core):
//   go run ./scripts/user_stream_check
//
// Make sure corresponding API keys are set in .env and enabled in config.

func main() {
	log.Println("=== User Stream check starting ===")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := events.NewBus()

	database, err := db.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("init DB error: %v", err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(database); err != nil {
		log.Fatalf("migrations error: %v", err)
	}

	log.Printf("Config: testnet=%v dryRun=%v", cfg.BinanceTestnet, cfg.DryRun)

	// Subscribe to filled events so we can see what the stream decoders emit.
	filledSub, unsubscribeFilled := bus.Subscribe(events.EventOrderFilled, 100)
	defer unsubscribeFilled()
	go func() {
		for msg := range filledSub {
			log.Printf("[EVENT] order filled: %#v", msg)
		}
	}()

	// Spot user stream
	if cfg.EnableBinanceTrading && cfg.BinanceAPIKey != "" && cfg.BinanceAPISecret != "" && !cfg.DryRun {
		log.Println("[SPOT] starting user stream listener...")
		spotClient := exspot.New(exspot.Config{
			APIKey:    cfg.BinanceAPIKey,
			APISecret: cfg.BinanceAPISecret,
			Testnet:   cfg.BinanceTestnet,
		})
		spotStream := order.NewSpotUserStream(spotClient, database, bus, cfg.BinanceTestnet)
		spotStream.Start(ctx)
	} else {
		log.Println("[SPOT] skipped (either disabled, missing key/secret, or DRY_RUN=true)")
	}

	// USDT‑M futures user stream
	if cfg.EnableBinanceUSDTFutures && cfg.BinanceUSDTKey != "" && cfg.BinanceUSDTSecret != "" && !cfg.DryRun {
		log.Println("[USDT] starting futures user stream listener...")
		usdtClient := exfutusdt.NewClient(exfutusdt.Config{
			APIKey:    cfg.BinanceUSDTKey,
			APISecret: cfg.BinanceUSDTSecret,
			Testnet:   cfg.BinanceTestnet,
		})
		usdtStream := order.NewFuturesUserStream(usdtClient, database, bus, cfg.BinanceTestnet, false)
		usdtStream.Start(ctx)
	} else {
		log.Println("[USDT] skipped (either disabled, missing key/secret, or DRY_RUN=true)")
	}

	// COIN‑M futures user stream
	if cfg.EnableBinanceCoinFutures && cfg.BinanceCoinKey != "" && cfg.BinanceCoinSecret != "" && !cfg.DryRun {
		log.Println("[COIN] starting futures user stream listener...")
		coinClient := exfutcoin.NewClient(exfutcoin.Config{
			APIKey:    cfg.BinanceCoinKey,
			APISecret: cfg.BinanceCoinSecret,
			Testnet:   cfg.BinanceTestnet,
		})
		coinStream := order.NewFuturesUserStream(coinClient, database, bus, cfg.BinanceTestnet, true)
		coinStream.Start(ctx)
	} else {
		log.Println("[COIN] skipped (either disabled, missing key/secret, or DRY_RUN=true)")
	}

	log.Println("User streams started. Place some test orders on Binance to see fill events.")

	// Wait for interrupt or timeout.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	select {
	case <-sigCh:
		log.Println("Interrupt received, shutting down user stream check...")
	case <-time.After(10 * time.Minute):
		log.Println("Timeout reached, stopping user stream check...")
	}

	cancel()
	time.Sleep(2 * time.Second)
	log.Println("=== User Stream check finished ===")
}
