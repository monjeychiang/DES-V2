package market

import (
	"context"
	"log"
	"math/rand"
	"time"

	"trading-core/internal/events"
)

// MockFeed generates synthetic ticks for local development.
type MockFeed struct {
	Bus        *events.Bus
	Symbols    []string
	StartPrice float64
	Step       float64
	Interval   time.Duration
}

func (m *MockFeed) Start(ctx context.Context) {
	if m.Bus == nil {
		log.Println("mock feed: bus not set")
		return
	}
	if len(m.Symbols) == 0 {
		m.Symbols = []string{"BTCUSDT"}
	}
	price := m.StartPrice
	if price == 0 {
		price = 100.0
	}
	if m.Step == 0 {
		m.Step = 0.5
	}
	if m.Interval == 0 {
		m.Interval = time.Second
	}

	go func() {
		t := time.NewTicker(m.Interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				for _, sym := range m.Symbols {
					// simple random walk
					price += (rand.Float64()*2 - 1) * m.Step
					m.Bus.Publish(events.EventPriceTick, struct {
						Symbol string
						Close  float64
					}{Symbol: sym, Close: price})
				}
			}
		}
	}()
}
