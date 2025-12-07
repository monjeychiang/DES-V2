//go:build ignore

// Package strategy usage examples

package main

import (
	"context"
	"log"

	"trading-core/internal/events"
	"trading-core/internal/market"
	"trading-core/internal/strategy"
	marketpkg "trading-core/pkg/market/binance"
)

// Example 1: MA Cross Strategy
func exampleMACross() {
	bus := events.NewBus()

	// Create MA Cross strategy
	// Fast MA: 10, Slow MA: 30, Size: 0.001 BTC
	maStrategy := strategy.NewMACrossStrategy("BTCUSDT", 10, 30, 0.001)

	// Create strategy engine
	engine := strategy.NewEngine(bus, strategy.Context{})
	engine.Add(maStrategy)

	// Subscribe to price data
	priceStream, _ := bus.Subscribe(events.EventPriceTick, 100)

	// Start strategy engine
	ctx := context.Background()
	engine.Start(ctx, priceStream)

	log.Println("MA Cross strategy started")
}

// Example 2: RSI Strategy
func exampleRSI() {
	bus := events.NewBus()

	// Create RSI strategy
	// Period: 14, Oversold: 30, Overbought: 70, Size: 0.001 BTC
	rsiStrategy := strategy.NewRSIStrategy("BTCUSDT", 14, 30, 70, 0.001)

	engine := strategy.NewEngine(bus, strategy.Context{})
	engine.Add(rsiStrategy)

	priceStream, _ := bus.Subscribe(events.EventPriceTick, 100)

	ctx := context.Background()
	engine.Start(ctx, priceStream)

	log.Println("RSI strategy started")
}

// Example 3: Bollinger Bands Strategy
func exampleBollinger() {
	bus := events.NewBus()

	// Create Bollinger strategy
	// Period: 20, StdDev: 2.0, Size: 0.001 BTC
	bbStrategy := strategy.NewBollingerStrategy("BTCUSDT", 20, 2.0, 0.001)

	engine := strategy.NewEngine(bus, strategy.Context{})
	engine.Add(bbStrategy)

	priceStream, _ := bus.Subscribe(events.EventPriceTick, 100)

	ctx := context.Background()
	engine.Start(ctx, priceStream)

	log.Println("Bollinger Bands strategy started")
}

// Example 4: Multiple Strategies
func exampleMultipleStrategies() {
	bus := events.NewBus()

	// Create multiple strategies
	maStrategy := strategy.NewMACrossStrategy("BTCUSDT", 10, 30, 0.001)
	rsiStrategy := strategy.NewRSIStrategy("BTCUSDT", 14, 30, 70, 0.001)
	bbStrategy := strategy.NewBollingerStrategy("BTCUSDT", 20, 2.0, 0.001)

	// Add all to engine
	engine := strategy.NewEngine(bus, strategy.Context{})
	engine.Add(maStrategy)
	engine.Add(rsiStrategy)
	engine.Add(bbStrategy)

	// Subscribe to signals
	signalStream, _ := bus.Subscribe(events.EventStrategySignal, 100)
	go func() {
		for sig := range signalStream {
			signal := sig.(strategy.Signal)
			log.Printf("📊 Signal: %s %s %.4f - %s",
				signal.Action, signal.Symbol, signal.Size, signal.Note)
		}
	}()

	// Start market data
	feed := &market.Feed{
		Client:   marketpkg.NewMarketDataClient(false),
		Stream:   marketpkg.NewStreamClient(false),
		Bus:      bus,
		Symbols:  []string{"btcusdt"},
		Interval: "1m",
	}

	ctx := context.Background()
	feed.Start(ctx)

	// Start strategy engine
	priceStream, _ := bus.Subscribe(events.EventPriceTick, 100)
	engine.Start(ctx, priceStream)

	log.Println("All strategies started with live data")

	// Keep running
	select {}
}
