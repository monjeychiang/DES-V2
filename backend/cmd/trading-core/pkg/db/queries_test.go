package db

import (
	"context"
	"testing"
	"time"
)

func TestUserQueriesRequireUserID(t *testing.T) {
	// Setup in-memory database
	database, err := New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	if err := ApplyMigrations(database); err != nil {
		t.Fatalf("Failed to apply migrations: %v", err)
	}

	q := database.Queries()
	ctx := context.Background()

	// Test: GetPositionsByUser requires user_id
	t.Run("GetPositionsByUser requires userID", func(t *testing.T) {
		_, err := q.GetPositionsByUser(ctx, "")
		if err != ErrUserIDRequired {
			t.Errorf("expected ErrUserIDRequired, got %v", err)
		}
	})

	// Test: GetOrdersByUser requires user_id
	t.Run("GetOrdersByUser requires userID", func(t *testing.T) {
		_, err := q.GetOrdersByUser(ctx, "", 100)
		if err != ErrUserIDRequired {
			t.Errorf("expected ErrUserIDRequired, got %v", err)
		}
	})

	// Test: GetTradesByUser requires user_id
	t.Run("GetTradesByUser requires userID", func(t *testing.T) {
		_, err := q.GetTradesByUser(ctx, "", 100)
		if err != ErrUserIDRequired {
			t.Errorf("expected ErrUserIDRequired, got %v", err)
		}
	})

	// Test: GetConnectionsByUser requires user_id
	t.Run("GetConnectionsByUser requires userID", func(t *testing.T) {
		_, err := q.GetConnectionsByUser(ctx, "")
		if err != ErrUserIDRequired {
			t.Errorf("expected ErrUserIDRequired, got %v", err)
		}
	})
}

func TestUserQueriesDataIsolation(t *testing.T) {
	// Setup in-memory database
	database, err := New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	if err := ApplyMigrations(database); err != nil {
		t.Fatalf("Failed to apply migrations: %v", err)
	}

	q := database.Queries()
	ctx := context.Background()

	userA := "user-a-123"
	userB := "user-b-456"

	// Insert orders for both users
	orderA := Order{
		ID:        "order-a-1",
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Price:     50000,
		Qty:       0.1,
		Status:    "NEW",
		UserID:    userA,
		CreatedAt: time.Now(),
	}
	orderB := Order{
		ID:        "order-b-1",
		Symbol:    "ETHUSDT",
		Side:      "SELL",
		Price:     3000,
		Qty:       1.0,
		Status:    "NEW",
		UserID:    userB,
		CreatedAt: time.Now(),
	}

	if err := q.CreateOrderWithUser(ctx, orderA); err != nil {
		t.Fatalf("Failed to create order A: %v", err)
	}
	if err := q.CreateOrderWithUser(ctx, orderB); err != nil {
		t.Fatalf("Failed to create order B: %v", err)
	}

	// Test: User A can only see User A's orders
	t.Run("User A sees only their orders", func(t *testing.T) {
		orders, err := q.GetOrdersByUser(ctx, userA, 100)
		if err != nil {
			t.Fatalf("Failed to get orders: %v", err)
		}
		if len(orders) != 1 {
			t.Errorf("expected 1 order, got %d", len(orders))
		}
		if len(orders) > 0 && orders[0].ID != "order-a-1" {
			t.Errorf("expected order-a-1, got %s", orders[0].ID)
		}
	})

	// Test: User B can only see User B's orders
	t.Run("User B sees only their orders", func(t *testing.T) {
		orders, err := q.GetOrdersByUser(ctx, userB, 100)
		if err != nil {
			t.Fatalf("Failed to get orders: %v", err)
		}
		if len(orders) != 1 {
			t.Errorf("expected 1 order, got %d", len(orders))
		}
		if len(orders) > 0 && orders[0].ID != "order-b-1" {
			t.Errorf("expected order-b-1, got %s", orders[0].ID)
		}
	})

	// Test: Random user sees no orders
	t.Run("Unknown user sees no orders", func(t *testing.T) {
		orders, err := q.GetOrdersByUser(ctx, "user-unknown", 100)
		if err != nil {
			t.Fatalf("Failed to get orders: %v", err)
		}
		if len(orders) != 0 {
			t.Errorf("expected 0 orders, got %d", len(orders))
		}
	})
}
