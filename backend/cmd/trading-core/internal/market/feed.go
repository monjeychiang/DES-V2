package market

import (
	"context"
	"log"
	"time"

	"trading-core/internal/events"
	market "trading-core/pkg/market/binance"
)

// Feed streams prices from Binance and publishes to the event bus.
type Feed struct {
	Client   *market.Client
	Stream   *market.StreamClient
	Bus      *events.Bus
	Symbols  []string
	Interval string
}

// Start begins polling + websocket streaming for configured symbols.
func (f *Feed) Start(ctx context.Context) {
	if f.Bus == nil || f.Client == nil || f.Stream == nil {
		log.Println("market feed not fully configured; skipping start")
		return
	}

	for _, sym := range f.Symbols {
		symbol := sym
		// Kick off websocket stream per symbol.
		ch, stop, err := f.Stream.SubscribeKlines(ctx, symbol, f.Interval)
		if err != nil {
			log.Printf("market feed: ws subscribe %s error: %v", symbol, err)
			continue
		}

		go func() {
			defer stop()
			for k := range ch {
				f.Bus.Publish(events.EventPriceTick, k)
			}
		}()
	}

	// Lightweight polling fallback to avoid gaps.
	go f.pollSnapshots(ctx)
}

func (f *Feed) pollSnapshots(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, sym := range f.Symbols {
				klines, err := f.Client.GetKlines(sym, f.Interval, 2, 0, 0)
				if err != nil {
					log.Printf("market feed snapshot %s error: %v", sym, err)
					continue
				}
				if len(klines) > 0 {
					f.Bus.Publish(events.EventPriceTick, klines[len(klines)-1])
				}
			}
		}
	}
}
