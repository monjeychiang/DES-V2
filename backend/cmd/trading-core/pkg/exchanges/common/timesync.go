package common

import (
	"context"
	"log"
	"sync"
	"time"
)

// TimeSync manages time synchronization with an exchange server.
type TimeSync struct {
	getServerTime func() (int64, error)
	offset        int64 // milliseconds offset (server - local)
	lastSync      time.Time
	syncInterval  time.Duration
	mu            sync.RWMutex
}

// NewTimeSync creates a new time synchronization manager.
func NewTimeSync(getServerTime func() (int64, error)) *TimeSync {
	return &TimeSync{
		getServerTime: getServerTime,
		syncInterval:  30 * time.Minute, // sync every 30 minutes
	}
}

// Start begins periodic time synchronization.
func (ts *TimeSync) Start(ctx context.Context) {
	// Initial sync
	if err := ts.Sync(ctx); err != nil {
		log.Printf("initial time sync failed: %v", err)
	}

	ticker := time.NewTicker(ts.syncInterval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := ts.Sync(ctx); err != nil {
					log.Printf("time sync failed: %v", err)
				}
			}
		}
	}()
}

// Sync synchronizes with server time.
func (ts *TimeSync) Sync(ctx context.Context) error {
	localBefore := time.Now().UnixMilli()
	serverTime, err := ts.getServerTime()
	if err != nil {
		return err
	}
	localAfter := time.Now().UnixMilli()

	// Assume network latency is symmetric
	networkLatency := (localAfter - localBefore) / 2
	localTime := localBefore + networkLatency

	ts.mu.Lock()
	ts.offset = serverTime - localTime
	ts.lastSync = time.Now()
	ts.mu.Unlock()

	log.Printf("time sync: offset=%dms, server=%d, local=%d", ts.offset, serverTime, localTime)
	return nil
}

// Now returns current time adjusted for server offset.
func (ts *TimeSync) Now() int64 {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return time.Now().UnixMilli() + ts.offset
}

// Offset returns the current time offset in milliseconds.
func (ts *TimeSync) Offset() int64 {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.offset
}
