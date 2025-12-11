package risk

import (
	"testing"
	"time"
)

// TestMultiUserManagerCleanupIdle verifies that CleanupIdle removes only idle managers.
func TestMultiUserManagerCleanupIdle(t *testing.T) {
	mgr := NewMultiUserManager(nil)

	// Create two users
	if _, err := mgr.GetOrCreate("userA"); err != nil {
		t.Fatalf("GetOrCreate userA: %v", err)
	}
	if _, err := mgr.GetOrCreate("userB"); err != nil {
		t.Fatalf("GetOrCreate userB: %v", err)
	}

	// Sanity check
	if got := mgr.UserCount(); got != 2 {
		t.Fatalf("expected 2 users before cleanup, got %d", got)
	}

	// Make userA look idle by moving its lastSeen far in the past.
	mgr.mu.Lock()
	mgr.lastSeen["userA"] = time.Now().Add(-2 * time.Hour)
	mgr.lastSeen["userB"] = time.Now()
	mgr.mu.Unlock()

	ttl := 1 * time.Hour
	mgr.CleanupIdle(ttl)

	if got := mgr.UserCount(); got != 1 {
		t.Fatalf("expected 1 user after cleanup, got %d", got)
	}
	if mgr.Get("userA") != nil {
		t.Fatalf("expected userA manager to be removed")
	}
	if mgr.Get("userB") == nil {
		t.Fatalf("expected userB manager to remain")
	}
}

// TestMultiUserManagerGetRefreshesLastSeen ensures read access counts as activity
// so cleanup does not evict recently accessed users.
func TestMultiUserManagerGetRefreshesLastSeen(t *testing.T) {
	mgr := NewMultiUserManager(nil)

	if _, err := mgr.GetOrCreate("activeUser"); err != nil {
		t.Fatalf("GetOrCreate activeUser: %v", err)
	}

	// Backdate the lastSeen timestamp to make the user appear idle.
	mgr.mu.Lock()
	mgr.lastSeen["activeUser"] = time.Now().Add(-2 * time.Hour)
	mgr.mu.Unlock()

	// A simple read should refresh lastSeen and prevent cleanup.
	if mgr.Get("activeUser") == nil {
		t.Fatalf("expected activeUser manager to be returned")
	}

	mgr.CleanupIdle(1 * time.Hour)

	if mgr.Get("activeUser") == nil {
		t.Fatalf("expected activeUser to remain after cleanup")
	}
}

// TestMultiUserManagerGetMissingDoesNotCreate ensures Get does not modify state
// when the user manager is absent.
func TestMultiUserManagerGetMissingDoesNotCreate(t *testing.T) {
	mgr := NewMultiUserManager(nil)

	if mgr.Get("missing") != nil {
		t.Fatalf("expected missing user to return nil manager")
	}

	if got := mgr.UserCount(); got != 0 {
		t.Fatalf("expected no managers to be created, found %d", got)
	}

	mgr.mu.RLock()
	_, ok := mgr.lastSeen["missing"]
	mgr.mu.RUnlock()
	if ok {
		t.Fatalf("expected lastSeen to remain untouched for missing user")
	}
}
