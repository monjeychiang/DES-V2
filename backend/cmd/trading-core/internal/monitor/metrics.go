package monitor

import (
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"trading-core/internal/gateway"
)

// SystemMetrics tracks overall system performance.
type SystemMetrics struct {
	mu sync.RWMutex

	// Latency histograms
	OrderLatency    *LatencyHistogram
	StrategyLatency *LatencyHistogram
	DBLatency       *LatencyHistogram

	// Counters
	ordersProcessed  uint64
	ticksProcessed   uint64
	signalsGenerated uint64
	errorsCount      uint64

	// Gateway pool & multi-user stats (updated periodically from main).
	gatewayStats     gateway.PoolStats
	riskActiveUsers  int
	balanceActiveUsers int

	// Snapshot
	lastUpdate time.Time
}

// LatencyHistogram tracks latency samples with sliding window.
// Supports lazy stats computation for better performance (V2 P1-B).
type LatencyHistogram struct {
	mu          sync.Mutex
	samples     []float64
	maxSize     int
	dirty       bool         // Whether samples have changed since last Stats()
	cachedStats LatencyStats // Cached computed stats
}

// NewSystemMetrics creates a new metrics instance.
func NewSystemMetrics() *SystemMetrics {
	return &SystemMetrics{
		OrderLatency:    NewLatencyHistogram(1000),
		StrategyLatency: NewLatencyHistogram(1000),
		DBLatency:       NewLatencyHistogram(1000),
		lastUpdate:      time.Now(),
	}
}

// NewLatencyHistogram creates a sliding window histogram.
func NewLatencyHistogram(size int) *LatencyHistogram {
	if size <= 0 {
		size = 1000
	}
	return &LatencyHistogram{
		samples: make([]float64, 0, size),
		maxSize: size,
		dirty:   true,
	}
}

// Record adds a latency sample in milliseconds.
func (h *LatencyHistogram) Record(latencyMs float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.samples) >= h.maxSize {
		// Shift window: remove oldest
		h.samples = h.samples[1:]
	}
	h.samples = append(h.samples, latencyMs)
	h.dirty = true // Mark as dirty for lazy recomputation
}

// RecordDuration converts duration to ms and records.
func (h *LatencyHistogram) RecordDuration(d time.Duration) {
	h.Record(float64(d.Nanoseconds()) / 1e6)
}

// Stats returns min, max, avg, p50, p95, p99.
// Uses lazy computation - only recomputes when samples have changed.
func (h *LatencyHistogram) Stats() LatencyStats {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Return cached stats if samples haven't changed
	if !h.dirty && h.cachedStats.Count > 0 {
		return h.cachedStats
	}

	n := len(h.samples)
	if n == 0 {
		return LatencyStats{}
	}

	// Compute new stats
	sorted := make([]float64, n)
	copy(sorted, h.samples)
	sort.Float64s(sorted)

	var sum float64
	min, max := sorted[0], sorted[n-1]
	for _, v := range sorted {
		sum += v
	}

	h.cachedStats = LatencyStats{
		Min:   min,
		Max:   max,
		Avg:   sum / float64(n),
		P50:   sorted[n/2],
		P95:   sorted[int(float64(n)*0.95)],
		P99:   sorted[int(float64(n)*0.99)],
		Count: n,
	}
	h.dirty = false

	return h.cachedStats
}

// LatencyStats holds computed latency statistics.
type LatencyStats struct {
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Avg   float64 `json:"avg"`
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
	Count int     `json:"count"`
}

// IncrementOrders increments processed orders counter.
func (m *SystemMetrics) IncrementOrders() {
	atomic.AddUint64(&m.ordersProcessed, 1)
}

// IncrementTicks increments processed ticks counter.
func (m *SystemMetrics) IncrementTicks() {
	atomic.AddUint64(&m.ticksProcessed, 1)
}

// IncrementSignals increments generated signals counter.
func (m *SystemMetrics) IncrementSignals() {
	atomic.AddUint64(&m.signalsGenerated, 1)
}

// IncrementErrors increments error counter.
func (m *SystemMetrics) IncrementErrors() {
	atomic.AddUint64(&m.errorsCount, 1)
}

// Snapshot returns current metrics snapshot.
type MetricsSnapshot struct {
	OrderLatency     LatencyStats `json:"order_latency"`
	StrategyLatency  LatencyStats `json:"strategy_latency"`
	DBLatency        LatencyStats `json:"db_latency"`
	OrdersProcessed  uint64       `json:"orders_processed"`
	TicksProcessed   uint64       `json:"ticks_processed"`
	SignalsGenerated uint64       `json:"signals_generated"`
	ErrorsCount      uint64       `json:"errors_count"`
	GatewayPool      gateway.PoolStats `json:"gateway_pool"`
	RiskActiveUsers  int               `json:"risk_active_users"`
	BalanceActiveUsers int             `json:"balance_active_users"`
	GoroutineCount   int          `json:"goroutine_count"`
	HeapAlloc        uint64       `json:"heap_alloc_bytes"`
	HeapSys          uint64       `json:"heap_sys_bytes"`
	Timestamp        time.Time    `json:"timestamp"`
}

// GetSnapshot returns a point-in-time metrics snapshot.
func (m *SystemMetrics) GetSnapshot() MetricsSnapshot {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.mu.RLock()
	gwStats := m.gatewayStats
	riskUsers := m.riskActiveUsers
	balanceUsers := m.balanceActiveUsers
	m.mu.RUnlock()

	return MetricsSnapshot{
		OrderLatency:     m.OrderLatency.Stats(),
		StrategyLatency:  m.StrategyLatency.Stats(),
		DBLatency:        m.DBLatency.Stats(),
		OrdersProcessed:  atomic.LoadUint64(&m.ordersProcessed),
		TicksProcessed:   atomic.LoadUint64(&m.ticksProcessed),
		SignalsGenerated: atomic.LoadUint64(&m.signalsGenerated),
		ErrorsCount:      atomic.LoadUint64(&m.errorsCount),
		GatewayPool:      gwStats,
		RiskActiveUsers:  riskUsers,
		BalanceActiveUsers: balanceUsers,
		GoroutineCount:   runtime.NumGoroutine(),
		HeapAlloc:        memStats.HeapAlloc,
		HeapSys:          memStats.HeapSys,
		Timestamp:        time.Now(),
	}
}

// SetGatewayPoolStats updates gateway pool statistics.
func (m *SystemMetrics) SetGatewayPoolStats(stats gateway.PoolStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gatewayStats = stats
}

// SetMultiUserCounts updates active user counts for risk/balance managers.
func (m *SystemMetrics) SetMultiUserCounts(riskUsers, balanceUsers int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.riskActiveUsers = riskUsers
	m.balanceActiveUsers = balanceUsers
}

// Timer helps measure operation duration.
type Timer struct {
	start     time.Time
	histogram *LatencyHistogram
}

// NewTimer creates a timer that records to the given histogram.
func NewTimer(h *LatencyHistogram) *Timer {
	return &Timer{
		start:     time.Now(),
		histogram: h,
	}
}

// Stop records elapsed time to histogram.
func (t *Timer) Stop() time.Duration {
	elapsed := time.Since(t.start)
	if t.histogram != nil {
		t.histogram.RecordDuration(elapsed)
	}
	return elapsed
}
