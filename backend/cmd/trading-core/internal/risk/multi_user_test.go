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

