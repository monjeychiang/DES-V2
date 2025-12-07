package monitor

import (
	"context"
	"log"
	"time"

	"trading-core/internal/events"
)

// Monitor watches events and emits alerts.
type Monitor struct {
	Bus     *events.Bus
	AlertFn func(string)
}

func (m *Monitor) Start(ctx context.Context) {
	if m.Bus == nil || m.AlertFn == nil {
		log.Println("monitor not fully configured; skipping")
		return
	}
	stream, unsub := m.Bus.Subscribe(events.EventRiskAlert, 50)
	go func() {
		defer unsub()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-stream:
				if !ok {
					return
				}
				m.AlertFn(formatAlert(msg))
			}
		}
	}()
}

func formatAlert(msg any) string {
	return "[" + time.Now().Format(time.RFC3339) + "] " + toString(msg)
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return "alert triggered"
	}
}
