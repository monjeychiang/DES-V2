package order

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// PersistentQueue wraps Queue with Write-Ahead Log (WAL) for crash recovery.
// Orders are persisted to disk before processing, ensuring no data loss.
type PersistentQueue struct {
	queue      *Queue
	walPath    string
	walFile    *os.File
	mu         sync.Mutex
	metrics    PersistentQueueMetrics
	processing map[string]bool // Track orders being processed
	closed     bool
}

// PersistentQueueMetrics tracks persistence statistics.
type PersistentQueueMetrics struct {
	Written   uint64 // Orders written to WAL
	Recovered uint64 // Orders recovered on startup
	Completed uint64 // Orders marked complete
	Failed    uint64 // Write failures
}

// walEntry represents a single WAL entry.
type walEntry struct {
	Action    string    `json:"action"` // "ENQUEUE" or "COMPLETE"
	Order     Order     `json:"order"`
	Timestamp time.Time `json:"timestamp"`
}

// NewPersistentQueue creates a persistent queue with WAL at the specified path.
func NewPersistentQueue(walDir string, queueSize int) (*PersistentQueue, error) {
	if err := os.MkdirAll(walDir, 0755); err != nil {
		return nil, fmt.Errorf("create WAL directory: %w", err)
	}

	walPath := filepath.Join(walDir, "order_queue.wal")
	file, err := os.OpenFile(walPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open WAL file: %w", err)
	}

	pq := &PersistentQueue{
		queue:      NewQueue(queueSize),
		walPath:    walPath,
		walFile:    file,
		processing: make(map[string]bool),
	}

	return pq, nil
}

// Recover loads pending orders from WAL after restart.
// Should be called before Drain() to restore queue state.
func (pq *PersistentQueue) Recover() error {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	file, err := os.Open(pq.walPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No WAL file, nothing to recover
		}
		return fmt.Errorf("open WAL for recovery: %w", err)
	}
	defer file.Close()

	// Build state from WAL: track enqueued and completed orders
	enqueued := make(map[string]Order)
	completed := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for large orders

	for scanner.Scan() {
		var entry walEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			log.Printf("⚠️ WAL parse error (skipping): %v", err)
			continue
		}

		switch entry.Action {
		case "ENQUEUE":
			enqueued[entry.Order.ID] = entry.Order
		case "COMPLETE":
			completed[entry.Order.ID] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("WAL scan error: %w", err)
	}

	// Re-enqueue pending orders (enqueued but not completed)
	recoveredCount := 0
	for id, order := range enqueued {
		if !completed[id] {
			pq.processing[id] = true
			pq.queue.Enqueue(order)
			recoveredCount++
		}
	}

	atomic.AddUint64(&pq.metrics.Recovered, uint64(recoveredCount))
	if recoveredCount > 0 {
		log.Printf("🔄 Recovered %d pending orders from WAL", recoveredCount)
	}

	// Compact WAL by rewriting only pending entries
	if recoveredCount > 0 || len(completed) > 10 {
		if err := pq.compactWAL(enqueued, completed); err != nil {
			log.Printf("⚠️ WAL compaction failed: %v", err)
		}
	}

	return nil
}

// compactWAL rewrites WAL with only pending entries.
func (pq *PersistentQueue) compactWAL(enqueued map[string]Order, completed map[string]bool) error {
	tempPath := pq.walPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(tempFile)
	for id, order := range enqueued {
		if !completed[id] {
			entry := walEntry{
				Action:    "ENQUEUE",
				Order:     order,
				Timestamp: order.CreatedAt,
			}
			if err := encoder.Encode(entry); err != nil {
				tempFile.Close()
				os.Remove(tempPath)
				return err
			}
		}
	}

	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return err
	}
	tempFile.Close()

	// Close current WAL and replace
	pq.walFile.Close()
	if err := os.Rename(tempPath, pq.walPath); err != nil {
		return err
	}

	// Reopen WAL
	pq.walFile, err = os.OpenFile(pq.walPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	log.Printf("✓ WAL compacted: kept %d pending entries", len(enqueued)-len(completed))
	return nil
}

// Enqueue adds an order with WAL persistence.
func (pq *PersistentQueue) Enqueue(o Order) bool {
	pq.mu.Lock()
	if pq.closed {
		pq.mu.Unlock()
		return false
	}

	// Write to WAL first
	entry := walEntry{
		Action:    "ENQUEUE",
		Order:     o,
		Timestamp: time.Now(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		pq.mu.Unlock()
		atomic.AddUint64(&pq.metrics.Failed, 1)
		log.Printf("❌ WAL marshal failed: %v", err)
		return false
	}

	if _, err := pq.walFile.Write(append(data, '\n')); err != nil {
		pq.mu.Unlock()
		atomic.AddUint64(&pq.metrics.Failed, 1)
		log.Printf("❌ WAL write failed: %v", err)
		return false
	}

	// Sync to disk for durability
	if err := pq.walFile.Sync(); err != nil {
		pq.mu.Unlock()
		atomic.AddUint64(&pq.metrics.Failed, 1)
		log.Printf("❌ WAL sync failed: %v", err)
		return false
	}

	pq.processing[o.ID] = true
	atomic.AddUint64(&pq.metrics.Written, 1)
	pq.mu.Unlock()

	// Now enqueue to in-memory queue
	return pq.queue.Enqueue(o)
}

// MarkComplete marks an order as completed in WAL.
func (pq *PersistentQueue) MarkComplete(orderID string) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if !pq.processing[orderID] {
		return // Not tracked or already completed
	}

	entry := walEntry{
		Action:    "COMPLETE",
		Order:     Order{ID: orderID},
		Timestamp: time.Now(),
	}
	data, _ := json.Marshal(entry)
	pq.walFile.Write(append(data, '\n'))
	// Don't sync here for performance, accept potential duplicate on crash

	delete(pq.processing, orderID)
	atomic.AddUint64(&pq.metrics.Completed, 1)
}

// Drain processes orders with automatic completion tracking.
func (pq *PersistentQueue) Drain(ctx context.Context, handler func(Order)) {
	pq.queue.Drain(ctx, func(o Order) {
		handler(o)
		pq.MarkComplete(o.ID)
	})
}

// GetMetrics returns persistence metrics.
func (pq *PersistentQueue) GetMetrics() PersistentQueueMetrics {
	return PersistentQueueMetrics{
		Written:   atomic.LoadUint64(&pq.metrics.Written),
		Recovered: atomic.LoadUint64(&pq.metrics.Recovered),
		Completed: atomic.LoadUint64(&pq.metrics.Completed),
		Failed:    atomic.LoadUint64(&pq.metrics.Failed),
	}
}

// Len returns queue depth.
func (pq *PersistentQueue) Len() int {
	return pq.queue.Len()
}

// PendingNotional returns total notional value of pending orders.
// Delegates to underlying Queue implementation.
func (pq *PersistentQueue) PendingNotional() float64 {
	return pq.queue.PendingNotional()
}

// Close closes the persistent queue and WAL file.
func (pq *PersistentQueue) Close() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	pq.closed = true
	pq.queue.Close()
	if pq.walFile != nil {
		pq.walFile.Sync()
		pq.walFile.Close()
	}
	log.Printf("✓ PersistentQueue closed: written=%d completed=%d",
		atomic.LoadUint64(&pq.metrics.Written),
		atomic.LoadUint64(&pq.metrics.Completed))
}
