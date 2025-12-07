package events

import (
	"sync"
)

// Bus is a lightweight pub/sub broker using channels.
type Bus struct {
	mu   sync.RWMutex
	subs map[Event][]chan any
}

// NewBus creates an event bus.
func NewBus() *Bus {
	return &Bus{subs: make(map[Event][]chan any)}
}

// Subscribe registers a listener for an event and returns the channel and an unsubscribe function.
func (b *Bus) Subscribe(e Event, buffer int) (<-chan any, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan any, buffer)
	b.subs[e] = append(b.subs[e], ch)

	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.subs[e]
		for i, c := range subs {
			if c == ch {
				close(c)
				b.subs[e] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}

	return ch, unsub
}

// Publish fan-outs the payload to subscribers asynchronously to avoid blocking.
func (b *Bus) Publish(e Event, payload any) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs[e] {
		select {
		case ch <- payload:
		default:
			// drop if subscriber is slow; keep broker non-blocking
		}
	}
}
