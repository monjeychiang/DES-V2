//go:build stress
// +build stress

package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"trading-core/internal/gateway"
	"trading-core/pkg/crypto"
	"trading-core/pkg/db"
	exchange "trading-core/pkg/exchanges/common"
)

// fakeGateway is a lightweight Gateway implementation used for stress tests.
type fakeGateway struct{}

func (f *fakeGateway) SubmitOrder(ctx context.Context, req exchange.OrderRequest) (exchange.OrderResult, error) {
	return exchange.OrderResult{Status: exchange.StatusNew}, nil
}

func (f *fakeGateway) CancelOrder(ctx context.Context, symbol, exchangeOrderID string) error {
	return nil
}

// TestMultiUserGatewayPoolStress simulates many users/connections hitting the GatewayPool concurrently.
// It is guarded by the "stress" build tag and is not intended for normal CI runs.
func TestMultiUserGatewayPoolStress(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "stress.db")

	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	defer database.Close()

	if err := db.ApplyMigrations(database); err != nil {
		t.Fatalf("ApplyMigrations: %v", err)
	}

	// Prepare KeyManager for encrypted connections (generate a valid test key).
	testKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	_ = os.Setenv("MASTER_ENCRYPTION_KEY", testKey)
	keyMgr, err := crypto.NewKeyManager()
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}

	userQueries := db.NewUserQueries(database.DB)

	// Create a number of users and encrypted connections.
	numUsers := 50
	connsPerUser := 3

	userIDs := make([]string, numUsers)
	userConnIDs := make([][]string, numUsers)

	for u := 0; u < numUsers; u++ {
		userID := uuid.NewString()
		userIDs[u] = userID
		userConnIDs[u] = make([]string, 0, connsPerUser)

		_, err := database.DB.Exec(`INSERT INTO users (id, email, password_hash) VALUES (?, ?, ?)`,
			userID, userID+"@example.com", "hash")
		if err != nil {
			t.Fatalf("insert user: %v", err)
		}

		for c := 0; c < connsPerUser; c++ {
			connID := uuid.NewString()
			userConnIDs[u] = append(userConnIDs[u], connID)

			apiKeyEnc, err := keyMgr.Encrypt("test-api-key")
			if err != nil {
				t.Fatalf("encrypt api key: %v", err)
			}
			apiSecretEnc, err := keyMgr.Encrypt("test-api-secret")
			if err != nil {
				t.Fatalf("encrypt api secret: %v", err)
			}

			conn := db.Connection{
				ID:                 connID,
				UserID:             userID,
				ExchangeType:       "binance-spot",
				Name:               "stress-conn",
				APIKeyEncrypted:    apiKeyEnc,
				APISecretEncrypted: apiSecretEnc,
				KeyVersion:         keyMgr.CurrentVersion(),
				IsActive:           true,
			}
			if err := userQueries.CreateConnectionEncrypted(ctx, conn); err != nil {
				t.Fatalf("CreateConnectionEncrypted: %v", err)
			}
		}
	}

	// Gateway factory that returns a cheap fake gateway.
	factory := func(conn db.Connection, apiKey, apiSecret string) (exchange.Gateway, error) {
		return &fakeGateway{}, nil
	}

	poolCfg := gateway.DefaultConfig()
	poolCfg.MaxSize = numUsers * connsPerUser / 2 // force LRU / eviction under load

	pool := gateway.NewManager(userQueries, keyMgr, factory, poolCfg)
	pool.Start(ctx)
	defer pool.Stop()

	// Concurrently hit the pool.
	var wg sync.WaitGroup
	workers := 20
	loops := 50

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < loops; i++ {
				u := (workerID*numUsers + i) % numUsers
				userID := userIDs[u]

				for c := 0; c < connsPerUser; c++ {
					if c >= len(userConnIDs[u]) {
						continue
					}
					connID := userConnIDs[u][c]

					_, err := pool.GetOrCreate(ctx, userID, connID)
					if err != nil && err != gateway.ErrConnectionNotFound && err != gateway.ErrGatewayUnhealthy {
						t.Errorf("GetOrCreate error: %v", err)
					}
				}
			}
		}(w)
	}

	wg.Wait()
}
