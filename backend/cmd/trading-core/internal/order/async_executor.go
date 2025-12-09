package order

import (
	"context"
	"errors"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// AsyncExecutor wraps Executor for non-blocking order execution.
type AsyncExecutor struct {
	executor     *Executor
	dryRunner    *DryRunExecutor
	resultCh     chan ExecutionResult
	workerPool   chan struct{}
	wg           sync.WaitGroup
	closed       bool
	mu           sync.Mutex
	maxRetries   int           // Maximum retry attempts (default: 3)
	retryBackoff time.Duration // Initial backoff duration (default: 100ms)
}

// ExecutionResult represents the outcome of an order execution.
type ExecutionResult struct {
	OrderID    string        `json:"order_id"`
	Success    bool          `json:"success"`
	Error      error         `json:"-"`
	ErrorMsg   string        `json:"error,omitempty"`
	Latency    time.Duration `json:"latency_ms"`
	Timestamp  time.Time     `json:"timestamp"`
	RetryCount int           `json:"retry_count,omitempty"`
}

// NewAsyncExecutor creates an async executor with specified worker count.
func NewAsyncExecutor(executor *Executor, workers int) *AsyncExecutor {
	if workers <= 0 {
		workers = 4
	}
	return &AsyncExecutor{
		executor:     executor,
		resultCh:     make(chan ExecutionResult, 100),
		workerPool:   make(chan struct{}, workers),
		maxRetries:   3,
		retryBackoff: 100 * time.Millisecond,
	}
}

// NewAsyncExecutorWithDryRun creates an async executor using DryRunExecutor.
func NewAsyncExecutorWithDryRun(dryRunner *DryRunExecutor, workers int) *AsyncExecutor {
	if workers <= 0 {
		workers = 4
	}
	return &AsyncExecutor{
		dryRunner:    dryRunner,
		resultCh:     make(chan ExecutionResult, 100),
		workerPool:   make(chan struct{}, workers),
		maxRetries:   3,
		retryBackoff: 100 * time.Millisecond,
	}
}

// isRetryableError checks if an error is transient and can be retried.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Retry on network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	// Retry on specific error messages
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"EOF",
		"i/o timeout",
	}
	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}
	return false
}

// ExecuteAsync submits an order for asynchronous execution with retry.
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
		retryCount := 0

		// Execute with retry logic
		for attempt := 0; attempt <= a.maxRetries; attempt++ {
			if attempt > 0 {
				backoff := a.retryBackoff * time.Duration(1<<(attempt-1)) // Exponential backoff
				log.Printf("🔄 Retrying order %s (attempt %d/%d) after %v", order.ID, attempt, a.maxRetries, backoff)
				select {
				case <-ctx.Done():
					err = ctx.Err()
					break
				case <-time.After(backoff):
				}
			}

			// Execute using appropriate executor
			if a.dryRunner != nil {
				err = a.dryRunner.Execute(ctx, order)
			} else if a.executor != nil {
				err = a.executor.Handle(ctx, order)
			} else {
				log.Printf("❌ No executor configured for order: %s", order.ID)
				return
			}

			// Success or non-retryable error
			if err == nil || !isRetryableError(err) {
				break
			}
			retryCount = attempt + 1
		}

		result := ExecutionResult{
			OrderID:    order.ID,
			Success:    err == nil,
			Error:      err,
			Latency:    time.Since(start),
			Timestamp:  time.Now(),
			RetryCount: retryCount,
		}

		if err != nil {
			result.ErrorMsg = err.Error()
			if retryCount > 0 {
				log.Printf("❌ Order %s failed after %d retries: %v (latency: %v)", order.ID, retryCount, err, result.Latency)
			} else {
				log.Printf("❌ Order %s failed: %v (latency: %v)", order.ID, err, result.Latency)
			}
		} else {
			if retryCount > 0 {
				log.Printf("✅ Order %s executed after %d retries (latency: %v)", order.ID, retryCount, result.Latency)
			} else {
				log.Printf("✅ Order %s executed (latency: %v)", order.ID, result.Latency)
			}
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

// SetRetryConfig updates retry configuration at runtime.
func (a *AsyncExecutor) SetRetryConfig(maxRetries int, backoff time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if maxRetries >= 0 {
		a.maxRetries = maxRetries
	}
	if backoff > 0 {
		a.retryBackoff = backoff
	}
}

// Close gracefully shuts down the async executor.
func (a *AsyncExecutor) Close() {
	a.mu.Lock()
	a.closed = true
	a.mu.Unlock()

	a.wg.Wait()
	close(a.resultCh)
}
