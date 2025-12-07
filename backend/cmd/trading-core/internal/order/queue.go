package order

import "context"

// Queue buffers orders before execution.
type Queue struct {
	ch chan Order
}

func NewQueue(size int) *Queue {
	if size <= 0 {
		size = 100
	}
	return &Queue{ch: make(chan Order, size)}
}

func (q *Queue) Enqueue(o Order) {
	q.ch <- o
}

func (q *Queue) Chan() <-chan Order {
	return q.ch
}

func (q *Queue) Close() {
	close(q.ch)
}

// Drain consumes orders with a handler until context is canceled.
func (q *Queue) Drain(ctx context.Context, handler func(Order)) {
	for {
		select {
		case <-ctx.Done():
			return
		case o, ok := <-q.ch:
			if !ok {
				return
			}
			handler(o)
		}
	}
}
