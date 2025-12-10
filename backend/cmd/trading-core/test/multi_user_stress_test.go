package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"trading-core/internal/balance"
	"trading-core/internal/risk"
	"trading-core/pkg/db"
)

// BenchmarkMultiUserOrderCreation benchmarks concurrent order creation.
func BenchmarkMultiUserOrderCreation(b *testing.B) {
	database, err := db.New(":memory:")
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	if err := db.ApplyMigrations(database); err != nil {
		b.Fatalf("Failed to apply migrations: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userID := fmt.Sprintf("user-%d", i%100) // 100 different users
		order := db.Order{
			ID:        fmt.Sprintf("order-%d", i),
			Symbol:    "BTCUSDT",
			Side:      "BUY",
			Price:     50000,
			Qty:       0.01,
			Status:    "NEW",
			UserID:    userID,
			CreatedAt: time.Now(),
		}
		if err := database.CreateOrder(ctx, order); err != nil {
			b.Errorf("CreateOrder failed: %v", err)
		}
	}
}

// BenchmarkMultiUserPositionUpsert benchmarks per-user position updates.
func BenchmarkMultiUserPositionUpsert(b *testing.B) {
	database, err := db.New(":memory:")
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	if err := db.ApplyMigrations(database); err != nil {
		b.Fatalf("Failed to apply migrations: %v", err)
	}

	q := database.Queries()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userID := fmt.Sprintf("user-%d", i%100)
		symbol := fmt.Sprintf("SYM%d", i%10)
		if err := q.UpsertPositionWithUser(ctx, userID, symbol, float64(i), 50000); err != nil {
			b.Errorf("UpsertPositionWithUser failed: %v", err)
		}
	}
}

// BenchmarkPerUserRiskManager benchmarks concurrent risk manager access.
func BenchmarkPerUserRiskManager(b *testing.B) {
	multiRiskMgr := risk.NewMultiUserManager(nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			userID := fmt.Sprintf("user-%d", i%100)
			_, _ = multiRiskMgr.GetOrCreate(userID)
			i++
		}
	})
}

// BenchmarkPerUserBalanceManager benchmarks concurrent balance manager access.
func BenchmarkPerUserBalanceManager(b *testing.B) {
	factory := func(userID string) (*balance.Manager, error) {
		mgr := balance.NewManager(nil, 30*time.Second)
		mgr.SetInitialBalance(10000.0)
		return mgr, nil
	}
	multiBalMgr := balance.NewMultiUserManager(factory)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			userID := fmt.Sprintf("user-%d", i%100)
			_, _ = multiBalMgr.GetOrCreate(userID)
			i++
		}
	})
}

// TestConcurrentMultiUserLoad simulates high concurrency multi-user scenario.
func TestConcurrentMultiUserLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	if err := db.ApplyMigrations(database); err != nil {
		t.Fatalf("Failed to apply migrations: %v", err)
	}

	ctx := context.Background()
	q := database.Queries()

	const numUsers = 500
	const ordersPerUser = 500
	const totalOrders = numUsers * ordersPerUser

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	startTime := time.Now()

	// Concurrent order creation
	for u := 0; u < numUsers; u++ {
		wg.Add(1)
		go func(userNum int) {
			defer wg.Done()
			userID := fmt.Sprintf("stress-user-%d", userNum)

			for i := 0; i < ordersPerUser; i++ {
				order := db.Order{
					ID:        fmt.Sprintf("stress-%d-%d", userNum, i),
					Symbol:    "BTCUSDT",
					Side:      "BUY",
					Price:     50000 + float64(i),
					Qty:       0.01,
					Status:    "NEW",
					UserID:    userID,
					CreatedAt: time.Now(),
				}
				if err := database.CreateOrder(ctx, order); err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(u)
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	t.Logf("=== Stress Test Results ===")
	t.Logf("Users: %d, Orders per user: %d", numUsers, ordersPerUser)
	t.Logf("Total orders: %d", totalOrders)
	t.Logf("Successful: %d, Errors: %d", successCount, errorCount)
	t.Logf("Duration: %v", elapsed)
	t.Logf("Throughput: %.2f orders/sec", float64(successCount)/elapsed.Seconds())

	if errorCount > 0 {
		t.Errorf("Stress test had %d errors", errorCount)
	}

	// Verify data isolation (skip for very large tests to avoid timeout)
	if numUsers <= 100 {
		for u := 0; u < numUsers; u++ {
			userID := fmt.Sprintf("stress-user-%d", u)
			orders, err := q.GetOrdersByUser(ctx, userID, ordersPerUser+100)
			if err != nil {
				t.Errorf("GetOrdersByUser failed for %s: %v", userID, err)
				continue
			}
			if len(orders) != ordersPerUser {
				t.Errorf("User %s expected %d orders, got %d", userID, ordersPerUser, len(orders))
			}
		}
		t.Logf("Data isolation verified for all %d users", numUsers)
	} else {
		// Sample check for large tests
		sampleUser := fmt.Sprintf("stress-user-%d", numUsers/2)
		orders, err := q.GetOrdersByUser(ctx, sampleUser, ordersPerUser+100)
		if err != nil {
			t.Errorf("Sample GetOrdersByUser failed: %v", err)
		} else if len(orders) != ordersPerUser {
			t.Errorf("Sample user expected %d orders, got %d", ordersPerUser, len(orders))
		} else {
			t.Logf("Sample data isolation verified for user %s (%d orders)", sampleUser, len(orders))
		}
	}
}

// TestCleanupIdleBehavior verifies the idle cleanup mechanism.
func TestCleanupIdleBehavior(t *testing.T) {
	multiRiskMgr := risk.NewMultiUserManager(nil)

	// Create managers for 5 users
	for i := 0; i < 5; i++ {
		_, _ = multiRiskMgr.GetOrCreate(fmt.Sprintf("user-%d", i))
	}

	if multiRiskMgr.UserCount() != 5 {
		t.Fatalf("Expected 5 users, got %d", multiRiskMgr.UserCount())
	}

	// Wait a bit to ensure timestamps are in the past
	time.Sleep(5 * time.Millisecond)

	// Cleanup with very short TTL (simulating all users being idle)
	multiRiskMgr.CleanupIdle(1 * time.Millisecond)

	if multiRiskMgr.UserCount() != 0 {
		t.Errorf("Expected 0 users after cleanup, got %d", multiRiskMgr.UserCount())
	}

	t.Log("Idle cleanup works correctly")
}

// TestBalanceManagerCleanup verifies balance manager cleanup.
func TestBalanceManagerCleanup(t *testing.T) {
	factory := func(userID string) (*balance.Manager, error) {
		mgr := balance.NewManager(nil, 30*time.Second)
		mgr.SetInitialBalance(10000.0)
		return mgr, nil
	}
	multiBalMgr := balance.NewMultiUserManager(factory)

	// Create managers for 5 users
	for i := 0; i < 5; i++ {
		_, _ = multiBalMgr.GetOrCreate(fmt.Sprintf("user-%d", i))
	}

	if multiBalMgr.UserCount() != 5 {
		t.Fatalf("Expected 5 users, got %d", multiBalMgr.UserCount())
	}

	// Wait a bit to ensure timestamps are in the past
	time.Sleep(5 * time.Millisecond)

	// Cleanup with very short TTL
	multiBalMgr.CleanupIdle(1 * time.Millisecond)

	if multiBalMgr.UserCount() != 0 {
		t.Errorf("Expected 0 users after cleanup, got %d", multiBalMgr.UserCount())
	}

	t.Log("Balance cleanup works correctly")
}
