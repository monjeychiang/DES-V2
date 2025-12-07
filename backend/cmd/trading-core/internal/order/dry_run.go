package order

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"trading-core/internal/events"
	"trading-core/pkg/db"
)

// ExecutionMode controls real vs dry-run.
type ExecutionMode int

const (
	ModeProduction ExecutionMode = iota
	ModeDryRun
)

// DryRunExecutor wraps a real Executor with a mock one.
type DryRunExecutor struct {
	mode     ExecutionMode
	realExec *Executor
	mockExec *MockExecutor
}

func NewDryRunExecutor(mode ExecutionMode, real *Executor, initialBalance float64) *DryRunExecutor {
	return &DryRunExecutor{
		mode:     mode,
		realExec: real,
		mockExec: NewMockExecutor(initialBalance),
	}
}

// Execute routes orders to either real or mock executor.
func (d *DryRunExecutor) Execute(ctx context.Context, o Order) error {
	if d.mode == ModeDryRun {
		// 1) Persist order to DB and emit order events, but do NOT hit exchange.
		if d.realExec != nil {
			// Temporarily skip any external gateway so Executor.Handle only stores to DB.
			d.realExec.SkipExchange = true
			// We ignore error here since Handle might fail if gateway lookup fails (which is fine in dry run skipping)
			// But if DB fails, we should probably know.
			// Currently Handle returns error if gateway lookup fails but we want to ignore that in DryRun?
			// Actually Handle checks SkipExchange first and doesn't lookup gateway.
			// So if Handle fails here, it's likely DB error.
			if err := d.realExec.Handle(ctx, o); err != nil {
				log.Printf("DRY-RUN: Warning, persistence failed: %v", err)
				// We don't block dry-run execution on DB failure, maybe?
				// But let's return error if we want to be strict.
				// For now let's just log and continue simulation.
			}
			d.realExec.SkipExchange = false
		}

		// 2) Run in-memory simulation for PnL / balance / positions.
		if err := d.mockExec.Execute(o); err != nil {
			fmt.Printf("DRY-RUN execute error: %v\n", err)
			return err
		}

		// 3) Store a synthetic trade + emit filled event to exercise downstream logic.
		if d.realExec != nil && d.realExec.DB != nil {
			trade := db.Trade{
				ID:        uuid.NewString(),
				OrderID:   o.ID,
				Symbol:    o.Symbol,
				Side:      o.Side,
				Price:     o.Price,
				Qty:       o.Qty,
				Fee:       0,
				CreatedAt: time.Now(),
			}
			if err := d.realExec.DB.CreateTrade(ctx, trade); err != nil {
				fmt.Printf("DRY-RUN store trade error: %v\n", err)
			}
		}
		if d.realExec != nil && d.realExec.Bus != nil {
			d.realExec.Bus.Publish(events.EventOrderFilled, struct {
				ID     string
				Symbol string
				Side   string
				Qty    float64
				Price  float64
			}{
				ID:     o.ID,
				Symbol: o.Symbol,
				Side:   o.Side,
				Qty:    o.Qty,
				Price:  o.Price,
			})
		}
		return nil
	}
	return d.realExec.Handle(ctx, o)
}

// PrintState prints current mock positions and balance for inspection.
func (d *DryRunExecutor) PrintState() {
	if d.mode != ModeDryRun || d.mockExec == nil {
		return
	}
	d.mockExec.printState()
}

// MockExecutor simulates order execution and simple PnL.
type MockExecutor struct {
	positions map[string]*MockPosition
	balance   float64
	orders    []MockOrder
	mu        sync.RWMutex
}

type MockPosition struct {
	Symbol     string
	Side       string
	Quantity   float64
	EntryPrice float64
}

type MockOrder struct {
	ID        string
	Symbol    string
	Side      string
	Quantity  float64
	Price     float64
	Status    string
	CreatedAt time.Time
	FilledAt  *time.Time
}

func NewMockExecutor(initialBalance float64) *MockExecutor {
	return &MockExecutor{
		positions: make(map[string]*MockPosition),
		balance:   initialBalance,
	}
}

func (m *MockExecutor) Execute(o Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Use provided price; if zero, treat as market and skip balance check.
	orderValue := o.Qty * o.Price
	if o.Price > 0 && orderValue > m.balance && strings.ToUpper(o.Side) == "BUY" {
		return fmt.Errorf("insufficient balance: need %.2f, have %.2f", orderValue, m.balance)
	}

	now := time.Now()
	mockOrder := MockOrder{
		ID:        o.ID,
		Symbol:    o.Symbol,
		Side:      o.Side,
		Quantity:  o.Qty,
		Price:     o.Price,
		Status:    "FILLED",
		CreatedAt: now,
		FilledAt:  &now,
	}
	m.orders = append(m.orders, mockOrder)

	// Update position
	m.updatePosition(mockOrder)

	// Update balance (simple cash accounting)
	if o.Price > 0 {
		if strings.ToUpper(o.Side) == "BUY" {
			m.balance -= orderValue
		} else if strings.ToUpper(o.Side) == "SELL" {
			m.balance += orderValue
		}
	}

	fmt.Printf("DRY-RUN: %s %s qty=%.4f price=%.4f balance=%.2f\n",
		o.Side, o.Symbol, o.Qty, o.Price, m.balance)
	return nil
}

func (m *MockExecutor) updatePosition(o MockOrder) {
	pos, exists := m.positions[o.Symbol]
	if !exists {
		m.positions[o.Symbol] = &MockPosition{
			Symbol:     o.Symbol,
			Side:       o.Side,
			Quantity:   o.Quantity,
			EntryPrice: o.Price,
		}
		return
	}

	if o.Side == pos.Side {
		totalValue := pos.Quantity*pos.EntryPrice + o.Quantity*o.Price
		pos.Quantity += o.Quantity
		if pos.Quantity != 0 {
			pos.EntryPrice = totalValue / pos.Quantity
		}
	} else {
		pos.Quantity -= o.Quantity
		if pos.Quantity <= 0 {
			delete(m.positions, o.Symbol)
		}
	}
}

func (m *MockExecutor) printState() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	fmt.Printf("DRY-RUN STATE: balance=%.2f\n", m.balance)
	for sym, pos := range m.positions {
		fmt.Printf("  pos %s side=%s qty=%.4f entry=%.4f\n", sym, pos.Side, pos.Quantity, pos.EntryPrice)
	}
	if len(m.positions) == 0 {
		fmt.Println("  (no open positions)")
	}
}
