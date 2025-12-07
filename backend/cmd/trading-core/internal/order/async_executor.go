package order

import (
	"context"
	"log"
	"sync"
	"time"
)

// AsyncExecutor wraps Executor for non-blocking order execution.
type AsyncExecutor struct {
	executor   *Executor
	dryRunner  *DryRunExecutor
	resultCh   chan ExecutionResult
	workerPool chan struct{}
	wg         sync.WaitGroup
	closed     bool
	mu         sync.Mutex
}

// ExecutionResult represents the outcome of an order execution.
type ExecutionResult struct {
	OrderID   string        `json:"order_id"`
	Success   bool          `json:"success"`
	Error     error         `json:"-"`
	ErrorMsg  string        `json:"error,omitempty"`
	Latency   time.Duration `json:"latency_ms"`
	Timestamp time.Time     `json:"timestamp"`
}

// NewAsyncExecutor creates an async executor with specified worker count.
func NewAsyncExecutor(executor *Executor, workers int) *AsyncExecutor {
	if workers <= 0 {
		workers = 4
	}
	return &AsyncExecutor{
		executor:   executor,
		resultCh:   make(chan ExecutionResult, 100),
		workerPool: make(chan struct{}, workers),
	}
}

// NewAsyncExecutorWithDryRun creates an async executor using DryRunExecutor.
func NewAsyncExecutorWithDryRun(dryRunner *DryRunExecutor, workers int) *AsyncExecutor {
	if workers <= 0 {
		workers = 4
	}
	return &AsyncExecutor{
		dryRunner:  dryRunner,
		resultCh:   make(chan ExecutionResult, 100),
		workerPool: make(chan struct{}, workers),
	}
}

// ExecuteAsync submits an order for asynchronous execution.
func (a *AsyncExecutor) ExecuteAsync(ctx context.Context, order Order) {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		log.Printf("❌ AsyncExecutor closed, order rejected: %s", order.ID)
		return
	}
	a.mu.Unlock()

	a.wg.Add(1)
	a.workerPool <- struct{}{} // Acquire worker slot

	go func() {
		defer a.wg.Done()
		defer func() { <-a.workerPool }() // Release worker slot

		start := time.Now()
		var err error

		// Execute using appropriate executor
		if a.dryRunner != nil {
			err = a.dryRunner.Execute(ctx, order)
		} else if a.executor != nil {
			err = a.executor.Handle(ctx, order)
		} else {
			log.Printf("❌ No executor configured for order: %s", order.ID)
			return
		}

		result := ExecutionResult{
			OrderID:   order.ID,
			Success:   err == nil,
			Error:     err,
			Latency:   time.Since(start),
			Timestamp: time.Now(),
		}

		if err != nil {
			result.ErrorMsg = err.Error()
			log.Printf("❌ Order %s failed: %v (latency: %v)", order.ID, err, result.Latency)
		} else {
			log.Printf("✅ Order %s executed (latency: %v)", order.ID, result.Latency)
		}

		// Send result (non-blocking)
		select {
		case a.resultCh <- result:
		default:
			log.Printf("⚠️ Result channel full, dropping result for %s", order.ID)
		}
	}()
}

// Results returns the result channel for monitoring.
func (a *AsyncExecutor) Results() <-chan ExecutionResult {
	return a.resultCh
}

// Pending returns the number of pending executions.
func (a *AsyncExecutor) Pending() int {
	return len(a.workerPool)
}

// WaitAll waits for all pending executions to complete.
func (a *AsyncExecutor) WaitAll() {
	a.wg.Wait()
}

// Close gracefully shuts down the async executor.
func (a *AsyncExecutor) Close() {
	a.mu.Lock()
	a.closed = true
	a.mu.Unlock()

	a.wg.Wait()
	close(a.resultCh)
}
