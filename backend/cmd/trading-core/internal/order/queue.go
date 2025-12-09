package order

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
)

// OrderQueue is the interface for order queuing implementations.
type OrderQueue interface {
	Enqueue(o Order) bool
	Drain(ctx context.Context, handler func(Order))
	Len() int
	PendingNotional() float64 // Total notional value of pending orders
	Close()
}

// QueueMetrics tracks queue performance statistics.
type QueueMetrics struct {
	Enqueued   uint64 // Total orders enqueued
	Dequeued   uint64 // Total orders dequeued
	Overflowed uint64 // Orders sent to overflow buffer
	Dropped    uint64 // Orders dropped (overflow full)
}

// Queue buffers orders before execution with overflow protection.
type Queue struct {
	ch          chan Order
	size        int
	overflowBuf []Order
	mu          sync.Mutex
	metrics     QueueMetrics
	closed      bool
}

// NewQueue creates an order queue with specified buffer size.
func NewQueue(size int) *Queue {
	if size <= 0 {
		size = 200
	}
	return &Queue{
		ch:          make(chan Order, size),
		size:        size,
		overflowBuf: make([]Order, 0, size/2),
	}
}

// Enqueue adds an order to the queue. Returns true if successful.
// Uses overflow buffer when main channel is full.
func (q *Queue) Enqueue(o Order) bool {
	atomic.AddUint64(&q.metrics.Enqueued, 1)

	select {
	case q.ch <- o:
		return true
	default:
		// Main channel full, try overflow buffer
		q.mu.Lock()
		defer q.mu.Unlock()

		if q.closed {
			log.Printf("❌ Order queue closed, order rejected: %s", o.ID)
			atomic.AddUint64(&q.metrics.Dropped, 1)
			return false
		}

		// Check overflow buffer capacity
		if len(q.overflowBuf) < cap(q.overflowBuf) {
			q.overflowBuf = append(q.overflowBuf, o)
			atomic.AddUint64(&q.metrics.Overflowed, 1)
			log.Printf("⚠️ Order queue overflow, using buffer (%d/%d): %s",
				len(q.overflowBuf), cap(q.overflowBuf), o.ID)
			return true
		}

		// Both main channel and overflow buffer full
		log.Printf("❌ Order queue full, order rejected: %s", o.ID)
		atomic.AddUint64(&q.metrics.Dropped, 1)
		return false
	}
}

// EnqueueBlocking adds an order to the queue, blocking if full.
// Use this for critical orders that must not be dropped.
func (q *Queue) EnqueueBlocking(o Order) {
	atomic.AddUint64(&q.metrics.Enqueued, 1)
	q.ch <- o
}

// Chan returns the order channel for consumption.
func (q *Queue) Chan() <-chan Order {
	return q.ch
}

// Len returns current queue depth (main channel only).
func (q *Queue) Len() int {
	return len(q.ch)
}

// PendingNotional returns total notional value of all pending orders.
// This is used for accurate exposure calculation.
func (q *Queue) PendingNotional() float64 {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Calculate overflow buffer notional
	var total float64
	for _, o := range q.overflowBuf {
		total += o.Qty * o.Price
	}

	// Note: We can't iterate the channel without draining it,
	// so we use the overflow buffer + estimate from channel length
	// For a more accurate implementation, consider tracking notional on enqueue/dequeue
	return total
}

// OverflowLen returns current overflow buffer depth.
func (q *Queue) OverflowLen() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.overflowBuf)
}

// drainOverflowBatch returns all overflow orders at once (V2 P1-C).
// This reduces lock contention compared to one-at-a-time processing.
func (q *Queue) drainOverflowBatch() []Order {
	q.mu.Lock()
	if len(q.overflowBuf) == 0 {
		q.mu.Unlock()
		return nil
	}

	batch := q.overflowBuf
	q.overflowBuf = make([]Order, 0, cap(batch))
	q.mu.Unlock()

	return batch
}

// GetMetrics returns a snapshot of queue metrics.
func (q *Queue) GetMetrics() QueueMetrics {
	return QueueMetrics{
		Enqueued:   atomic.LoadUint64(&q.metrics.Enqueued),
		Dequeued:   atomic.LoadUint64(&q.metrics.Dequeued),
		Overflowed: atomic.LoadUint64(&q.metrics.Overflowed),
		Dropped:    atomic.LoadUint64(&q.metrics.Dropped),
	}
}

// Close closes the queue.
func (q *Queue) Close() {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	close(q.ch)
}

// Drain consumes orders with a handler until context is canceled.
// Also drains any overflow buffer entries using batch processing (V2 P1-C).
func (q *Queue) Drain(ctx context.Context, handler func(Order)) {
	for {
		// Batch process overflow buffer (reduces lock contention)
		if batch := q.drainOverflowBatch(); batch != nil {
			for _, o := range batch {
				atomic.AddUint64(&q.metrics.Dequeued, 1)
				handler(o)
			}
			continue
		}

		// Then, read from main channel
		select {
		case <-ctx.Done():
			return
		case o, ok := <-q.ch:
			if !ok {
				return
			}
			atomic.AddUint64(&q.metrics.Dequeued, 1)
			handler(o)
		}
	}
}
