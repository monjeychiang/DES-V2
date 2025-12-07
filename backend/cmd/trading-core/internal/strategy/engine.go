package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"

	"trading-core/internal/data"
	"trading-core/internal/events"
	market "trading-core/pkg/market/binance"
)

// Engine orchestrates multiple strategies and emits signals on the event bus.
type Engine struct {
	strategies  []Strategy
	paused      map[string]bool // Set of paused strategy IDs
	bus         *events.Bus
	ctx         Context
	db          *sql.DB
	dataService *data.HistoricalDataService
}

func NewEngine(bus *events.Bus, db *sql.DB, ctx Context) *Engine {
	return &Engine{
		paused:      make(map[string]bool),
		bus:         bus,
		db:          db,
		ctx:         ctx,
		dataService: data.NewHistoricalDataService(false), // Default to mainnet for data
	}
}

// Add registers a strategy implementation.
func (e *Engine) Add(s Strategy) {
	e.strategies = append(e.strategies, s)
}

// LoadStrategies loads active strategies from the database.
func (e *Engine) LoadStrategies(db *sql.DB) error {
	// Load strategies that are ACTIVE or PAUSED
	rows, err := db.Query(`
		SELECT id, strategy_type, symbol, parameters, status
		FROM strategy_instances 
		WHERE status IN ('ACTIVE', 'PAUSED') OR (status IS NULL AND is_active = 1)
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	e.strategies = nil // Reset strategies
	e.paused = make(map[string]bool)

	for rows.Next() {
		var id, sType, symbol, status string
		var paramsJSON string
		// Handle potential NULL status by scanning into sql.NullString if needed,
		// but we used OR in query so we expect status to be populated or fallback.
		// Actually, let's just scan status. If it's NULL (old rows), it might fail if we don't handle it.
		// Let's assume schema migration set default 'ACTIVE'.
		if err := rows.Scan(&id, &sType, &symbol, &paramsJSON, &status); err != nil {
			return err
		}

		if status == "PAUSED" {
			e.paused[id] = true
		}

		var strategy Strategy

		switch sType {
		case "ma_cross":
			var p struct {
				FastPeriod int     `json:"fast"`
				SlowPeriod int     `json:"slow"`
				Size       float64 `json:"size"`
			}
			if err := json.Unmarshal([]byte(paramsJSON), &p); err != nil {
				log.Printf("failed to unmarshal params for %s: %v", id, err)
				continue
			}
			strategy = NewMACrossStrategy(id, symbol, p.FastPeriod, p.SlowPeriod, p.Size)

		case "rsi":
			var p struct {
				Period     int     `json:"period"`
				Oversold   float64 `json:"oversold"`
				Overbought float64 `json:"overbought"`
				Size       float64 `json:"size"`
			}
			if err := json.Unmarshal([]byte(paramsJSON), &p); err != nil {
				log.Printf("failed to unmarshal params for %s: %v", id, err)
				continue
			}
			strategy = NewRSIStrategy(id, symbol, p.Period, p.Oversold, p.Overbought, p.Size)

		case "bollinger":
			var p struct {
				Period    int     `json:"period"`
				NumStdDev float64 `json:"std_dev"`
				Size      float64 `json:"size"`
			}
			if err := json.Unmarshal([]byte(paramsJSON), &p); err != nil {
				log.Printf("failed to unmarshal params for %s: %v", id, err)
				continue
			}
			strategy = NewBollingerStrategy(id, symbol, p.Period, p.NumStdDev, p.Size)

		default:
			log.Printf("unknown strategy type: %s", sType)
			continue
		}

		if strategy != nil {
			e.Add(strategy)
			log.Printf("Loaded strategy: %s (%s)", strategy.Name(), id)
		}
	}
	return nil
}

// Start subscribes to price ticks and forwards to strategies.
func (e *Engine) Start(ctx context.Context, priceStream <-chan any) {
	// 1. Load state and warm up strategies
	for _, s := range e.strategies {
		// Load state
		var stateData string
		err := e.db.QueryRow("SELECT state_data FROM strategy_states WHERE strategy_instance_id = ?", s.ID()).Scan(&stateData)
		if err == nil {
			if err := s.SetState(json.RawMessage(stateData)); err != nil {
				log.Printf("⚠️ Failed to restore state for strategy %s: %v", s.Name(), err)
			} else {
				log.Printf("✓ Restored state for strategy %s", s.Name())
			}
		} else if err != sql.ErrNoRows {
			log.Printf("⚠️ DB error loading state for %s: %v", s.Name(), err)
		}

		// Warm up with historical data (e.g., last 100 candles)
		// Note: In a real scenario, we should determine required candles dynamically.
		// For now, we fetch 100 1h candles as a safe default for warm-up.
		// We need to know the symbol and interval from the strategy instance config,
		// but the Strategy interface doesn't expose Interval.
		// We can query it from DB or add it to interface.
		// For simplicity, let's query DB for interval.
		var interval string
		var symbol string
		_ = e.db.QueryRow("SELECT symbol, interval FROM strategy_instances WHERE id = ?", s.ID()).Scan(&symbol, &interval)

		if symbol != "" && interval != "" {
			klines, err := e.dataService.GetKlines(ctx, symbol, interval, 100)
			if err != nil {
				log.Printf("⚠️ Failed to fetch warm-up data for %s: %v", s.Name(), err)
			} else {
				log.Printf("🔥 Warming up %s with %d klines...", s.Name(), len(klines))
				for _, k := range klines {
					// Feed historical data silently (ignore signals)
					_, _ = s.OnTick(symbol, k.Close, nil)
				}
			}
		}
	}

	go func() {
		defer e.saveAllStates() // Save state on exit

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-priceStream:
				if !ok {
					return
				}
				e.handleTick(msg)
			}
		}
	}()
}

func (e *Engine) saveAllStates() {
	for _, s := range e.strategies {
		state, err := s.GetState()
		if err != nil {
			log.Printf("Failed to get state for %s: %v", s.Name(), err)
			continue
		}

		_, err = e.db.Exec(`
			INSERT INTO strategy_states (strategy_instance_id, state_data, updated_at)
			VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(strategy_instance_id) DO UPDATE SET
				state_data = excluded.state_data,
				updated_at = CURRENT_TIMESTAMP
		`, s.ID(), string(state))

		if err != nil {
			log.Printf("Failed to save state for %s: %v", s.Name(), err)
		}
	}
	log.Println("💾 All strategy states saved.")
}

func (e *Engine) handleTick(msg any) {
	symbol := ""
	price := 0.0

	switch v := msg.(type) {
	case market.Kline:
		symbol = ""
		price = v.Close
	case struct {
		Symbol string
		Close  float64
	}:
		symbol = v.Symbol
		price = v.Close
	}

	indVals := map[string]float64{}
	if e.ctx.Indicators != nil && price > 0 {
		indVals = e.ctx.Indicators.Update(symbol, price)
	}

	for _, s := range e.strategies {
		if e.paused[s.ID()] {
			continue // Skip paused strategies
		}

		sig, err := s.OnTick(symbol, price, indVals)
		if err != nil {
			log.Printf("strategy %s error: %v", s.Name(), err)
			continue
		}
		if sig != nil {
			sig.StrategyID = s.ID()
			log.Printf("strategy %s signal: %+v", s.Name(), sig)
			e.bus.Publish(events.EventStrategySignal, *sig)
		}
	}
}

// Lifecycle Methods

func (e *Engine) PauseStrategy(id string) error {
	e.paused[id] = true
	_, err := e.db.Exec("UPDATE strategy_instances SET status = 'PAUSED' WHERE id = ?", id)
	return err
}

func (e *Engine) ResumeStrategy(id string) error {
	delete(e.paused, id)
	_, err := e.db.Exec("UPDATE strategy_instances SET status = 'ACTIVE' WHERE id = ?", id)
	return err
}

func (e *Engine) StopStrategy(id string) error {
	// Remove from memory
	newStrategies := make([]Strategy, 0, len(e.strategies))
	for _, s := range e.strategies {
		if s.ID() != id {
			newStrategies = append(newStrategies, s)
		}
	}
	e.strategies = newStrategies
	delete(e.paused, id)

	// Update DB
	_, err := e.db.Exec("UPDATE strategy_instances SET status = 'STOPPED', is_active = 0 WHERE id = ?", id)
	return err
}

// GetStrategyPosition returns the current virtual position for a strategy.
func (e *Engine) GetStrategyPosition(id string) (float64, error) {
	var qty float64
	err := e.db.QueryRow("SELECT qty FROM strategy_positions WHERE strategy_instance_id = ?", id).Scan(&qty)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return qty, nil
}

// UpdateParams updates the strategy parameters and reloads the instance if active.
func (e *Engine) UpdateParams(id string, paramsJSON []byte) error {
	// 1. Update DB
	_, err := e.db.Exec("UPDATE strategy_instances SET parameters = ? WHERE id = ?", string(paramsJSON), id)
	if err != nil {
		return err
	}

	// 2. Reload if in memory
	// Find if currently loaded
	var current Strategy
	for _, s := range e.strategies {
		if s.ID() == id {
			current = s
			break
		}
	}

	if current != nil {
		// Remove old instance
		newStrategies := make([]Strategy, 0, len(e.strategies))
		for _, s := range e.strategies {
			if s.ID() != id {
				newStrategies = append(newStrategies, s)
			}
		}
		e.strategies = newStrategies

		// Re-load this specific strategy from DB to get fresh config and factory creation
		// We can reuse LoadStrategies logic but restricted to this ID?
		// Or just manually re-query this single row.
		return e.reloadSingleStrategy(id)
	}

	return nil
}

func (e *Engine) reloadSingleStrategy(id string) error {
	var sType, symbol, status string
	var paramsJSON string
	err := e.db.QueryRow(`
		SELECT strategy_type, symbol, parameters, status 
		FROM strategy_instances 
		WHERE id = ?`, id).Scan(&sType, &symbol, &paramsJSON, &status)
	if err != nil {
		return err
	}

	var strategy Strategy

	switch sType {
	case "ma_cross":
		var p struct {
			FastPeriod int     `json:"fast"`
			SlowPeriod int     `json:"slow"`
			Size       float64 `json:"size"`
		}
		if err := json.Unmarshal([]byte(paramsJSON), &p); err != nil {
			return err
		}
		strategy = NewMACrossStrategy(id, symbol, p.FastPeriod, p.SlowPeriod, p.Size)

	case "rsi":
		var p struct {
			Period     int     `json:"period"`
			Oversold   float64 `json:"oversold"`
			Overbought float64 `json:"overbought"`
			Size       float64 `json:"size"`
		}
		if err := json.Unmarshal([]byte(paramsJSON), &p); err != nil {
			return err
		}
		strategy = NewRSIStrategy(id, symbol, p.Period, p.Oversold, p.Overbought, p.Size)

	case "bollinger":
		var p struct {
			Period    int     `json:"period"`
			NumStdDev float64 `json:"std_dev"`
			Size      float64 `json:"size"`
		}
		if err := json.Unmarshal([]byte(paramsJSON), &p); err != nil {
			return err
		}
		strategy = NewBollingerStrategy(id, symbol, p.Period, p.NumStdDev, p.Size)
	}

	if strategy != nil {
		// Restore state
		var stateData string
		err := e.db.QueryRow("SELECT state_data FROM strategy_states WHERE strategy_instance_id = ?", id).Scan(&stateData)
		if err == nil {
			_ = strategy.SetState(json.RawMessage(stateData))
		}

		e.Add(strategy)
		if status == "PAUSED" {
			e.paused[id] = true
		}
		log.Printf("Reloaded strategy: %s", strategy.Name())
	}
	return nil
}
