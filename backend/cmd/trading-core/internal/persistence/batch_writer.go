package persistence

import (
	"database/sql"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// WriteOp represents a database write operation.
type WriteOp struct {
	Table string
	Query string
	Args  []any
}

// BatchWriter batches database writes for improved performance.
type BatchWriter struct {
	db          *sql.DB
	buffer      []WriteOp
	mu          sync.Mutex
	maxSize     int
	flushIntval time.Duration
	done        chan struct{}
	wg          sync.WaitGroup
	metrics     BatchWriterMetrics
}

// BatchWriterMetrics provides statistics about batch operations (V2 P1-D).
type BatchWriterMetrics struct {
	TotalWrites   uint64    `json:"total_writes"`
	TotalBatches  uint64    `json:"total_batches"`
	TotalErrors   uint64    `json:"total_errors"`
	LastBatchSize int       `json:"last_batch_size"`
	LastFlushTime time.Time `json:"last_flush_time"`
}

// NewBatchWriter creates a batch writer with specified parameters.
// maxSize: max operations before auto-flush
// interval: time-based flush interval
func NewBatchWriter(db *sql.DB, maxSize int, interval time.Duration) *BatchWriter {
	if maxSize <= 0 {
		maxSize = 50
	}
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}

	bw := &BatchWriter{
		db:          db,
		buffer:      make([]WriteOp, 0, maxSize),
		maxSize:     maxSize,
		flushIntval: interval,
		done:        make(chan struct{}),
	}

	bw.wg.Add(1)
	go bw.backgroundFlush()

	return bw
}

// Write adds a write operation to the batch.
func (bw *BatchWriter) Write(op WriteOp) {
	bw.mu.Lock()
	bw.buffer = append(bw.buffer, op)
	shouldFlush := len(bw.buffer) >= bw.maxSize
	bw.mu.Unlock()

	if shouldFlush {
		bw.Flush()
	}
}

// WriteQuery is a convenience method for simple queries.
func (bw *BatchWriter) WriteQuery(query string, args ...any) {
	bw.Write(WriteOp{
		Query: query,
		Args:  args,
	})
}

// Flush immediately writes all buffered operations to the database.
func (bw *BatchWriter) Flush() error {
	bw.mu.Lock()
	if len(bw.buffer) == 0 {
		bw.mu.Unlock()
		return nil
	}

	ops := bw.buffer
	bw.buffer = make([]WriteOp, 0, bw.maxSize)
	bw.mu.Unlock()

	return bw.executeBatch(ops)
}

// executeBatch runs a batch of operations in a transaction.
func (bw *BatchWriter) executeBatch(ops []WriteOp) error {
	if len(ops) == 0 {
		return nil
	}

	// Track metrics (V2 P1-D)
	atomic.AddUint64(&bw.metrics.TotalWrites, uint64(len(ops)))
	atomic.AddUint64(&bw.metrics.TotalBatches, 1)
	bw.metrics.LastBatchSize = len(ops)
	bw.metrics.LastFlushTime = time.Now()

	tx, err := bw.db.Begin()
	if err != nil {
		atomic.AddUint64(&bw.metrics.TotalErrors, 1)
		log.Printf("❌ BatchWriter: failed to begin transaction: %v", err)
		return err
	}

	for _, op := range ops {
		if _, err := tx.Exec(op.Query, op.Args...); err != nil {
			tx.Rollback()
			atomic.AddUint64(&bw.metrics.TotalErrors, 1)
			log.Printf("❌ BatchWriter: query failed, rolling back: %v", err)
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		atomic.AddUint64(&bw.metrics.TotalErrors, 1)
		log.Printf("❌ BatchWriter: commit failed: %v", err)
		return err
	}

	log.Printf("💾 BatchWriter: flushed %d operations", len(ops))
	return nil
}

// backgroundFlush periodically flushes the buffer.
func (bw *BatchWriter) backgroundFlush() {
	defer bw.wg.Done()
	ticker := time.NewTicker(bw.flushIntval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := bw.Flush(); err != nil {
				log.Printf("⚠️ BatchWriter: background flush error: %v", err)
			}
		case <-bw.done:
			// Final flush before shutdown
			if err := bw.Flush(); err != nil {
				log.Printf("⚠️ BatchWriter: final flush error: %v", err)
			}
			return
		}
	}
}

// Pending returns the number of pending operations.
func (bw *BatchWriter) Pending() int {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return len(bw.buffer)
}

// GetMetrics returns the current metrics for the batch writer (V2 P1-D).
func (bw *BatchWriter) GetMetrics() BatchWriterMetrics {
	return BatchWriterMetrics{
		TotalWrites:   atomic.LoadUint64(&bw.metrics.TotalWrites),
		TotalBatches:  atomic.LoadUint64(&bw.metrics.TotalBatches),
		TotalErrors:   atomic.LoadUint64(&bw.metrics.TotalErrors),
		LastBatchSize: bw.metrics.LastBatchSize,
		LastFlushTime: bw.metrics.LastFlushTime,
	}
}

// Close gracefully shuts down the batch writer.
func (bw *BatchWriter) Close() error {
	close(bw.done)
	bw.wg.Wait()
	return nil
}
