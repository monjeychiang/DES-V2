package reconciliation

import (
	"context"
	"log"
	"math"
	"sync"
	"time"

	"trading-core/internal/state"
	"trading-core/pkg/db"
)

// ExchangeClient interface for reconciliation
type ExchangeClient interface {
	GetPositions(ctx context.Context) (map[string]Position, error)
}

// Position from exchange
type Position struct {
	Symbol   string
	Quantity float64
}

// Service handles periodic reconciliation
type Service struct {
	exchange ExchangeClient
	stateMgr *state.Manager
	database *db.Database
	interval time.Duration
	autoSync bool // 是否自動同步
	mu       sync.Mutex
}

// ReconciliationReport contains reconciliation results
type ReconciliationReport struct {
	Timestamp     time.Time
	PositionDiffs []PositionDiff
	HasDiffs      bool
	SyncedCount   int // 自動同步的數量
}

// PositionDiff represents a position difference
type PositionDiff struct {
	Symbol      string
	LocalQty    float64
	ExchangeQty float64
	Difference  float64
	Synced      bool // 是否已同步
}

// NewService creates a new reconciliation service
func NewService(exchange ExchangeClient, stateMgr *state.Manager, database *db.Database, interval time.Duration) *Service {
	return &Service{
		exchange: exchange,
		stateMgr: stateMgr,
		database: database,
		interval: interval,
		autoSync: true, // 默認啟用自動同步
	}
}

// SetAutoSync enables or disables auto-sync
func (s *Service) SetAutoSync(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoSync = enabled
	log.Printf("📊 Reconciliation auto-sync: %v", enabled)
}

// Start begins periodic reconciliation
func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				report, err := s.Reconcile(ctx)
				if err != nil {
					log.Printf("❌ Reconciliation error: %v", err)
					continue
				}

				s.handleReport(ctx, report)

			case <-ctx.Done():
				return
			}
		}
	}()

	log.Printf("✓ Reconciliation service started (interval: %v, auto-sync: %v)", s.interval, s.autoSync)
}

// Reconcile performs reconciliation check
func (s *Service) Reconcile(ctx context.Context) (*ReconciliationReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.exchange == nil {
		// No exchange in dry-run mode
		return &ReconciliationReport{
			Timestamp: time.Now(),
			HasDiffs:  false,
		}, nil
	}

	report := &ReconciliationReport{
		Timestamp:     time.Now(),
		PositionDiffs: []PositionDiff{},
	}

	// Get exchange positions
	exchangePos, err := s.exchange.GetPositions(ctx)
	if err != nil {
		return nil, err
	}

	// Compare each exchange position with local
	for symbol, exPos := range exchangePos {
		localPos := s.stateMgr.Position(symbol)

		if math.Abs(localPos.Qty-exPos.Quantity) > 0.0001 {
			diff := PositionDiff{
				Symbol:      symbol,
				LocalQty:    localPos.Qty,
				ExchangeQty: exPos.Quantity,
				Difference:  localPos.Qty - exPos.Quantity,
				Synced:      false,
			}

			// Auto-sync if enabled
			if s.autoSync {
				if s.syncPosition(ctx, symbol, exPos.Quantity) {
					diff.Synced = true
					report.SyncedCount++
				}
			}

			report.PositionDiffs = append(report.PositionDiffs, diff)
			report.HasDiffs = true
		}
	}

	return report, nil
}

// syncPosition syncs local position to match exchange
func (s *Service) syncPosition(ctx context.Context, symbol string, exchangeQty float64) bool {
	// Get current local position
	localPos := s.stateMgr.Position(symbol)

	// Calculate the difference
	diff := exchangeQty - localPos.Qty

	if math.Abs(diff) < 0.0001 {
		return false // No sync needed
	}

	// Keep existing average price if available, otherwise use 0
	avgPrice := localPos.AvgPrice
	if avgPrice == 0 && exchangeQty != 0 {
		avgPrice = 1.0 // Placeholder price for positions without price history
	}

	// Record sync operation before updating
	log.Printf("🔄 Syncing position: %s from %.4f to %.4f (diff: %.4f)",
		symbol, localPos.Qty, exchangeQty, diff)

	// Actually update the position in state manager
	if err := s.stateMgr.SetPosition(ctx, symbol, exchangeQty, avgPrice); err != nil {
		log.Printf("❌ Failed to sync position %s: %v", symbol, err)
		return false
	}

	log.Printf("✅ Successfully synced position: %s to %.4f", symbol, exchangeQty)
	return true
}

// handleReport processes reconciliation report
func (s *Service) handleReport(ctx context.Context, report *ReconciliationReport) {
	if report.HasDiffs {
		log.Printf("⚠️ Reconciliation - Position differences detected:")
		for _, diff := range report.PositionDiffs {
			status := "❌ Not synced"
			if diff.Synced {
				status = "✅ Synced"
			}
			log.Printf("  %s: Local=%.4f, Exchange=%.4f, Diff=%.4f [%s]",
				diff.Symbol, diff.LocalQty, diff.ExchangeQty, diff.Difference, status)
		}

		if report.SyncedCount > 0 {
			log.Printf("🔄 Auto-synced %d positions", report.SyncedCount)
		}

		// Save report to database for audit trail
		s.saveReport(ctx, report)
	} else {
		log.Printf("✅ Reconciliation OK - All positions match")
	}
}

// saveReport saves reconciliation report to database (placeholder)
func (s *Service) saveReport(ctx context.Context, report *ReconciliationReport) {
	// TODO: Implement database save
	// This should create an audit trail of all reconciliation events
}
