package limiter

import (
	"sync"
	"testing"
	"time"
)

// 1. Test basic capacity and blocking
func TestRateLimiter_Capacity(t *testing.T) {
	limiter := NewIPRateLimiter(3.0, 1.0)
	testIP := "192.168.1.1"

	for i := 0; i < 3; i++ {
		if !limiter.Allow(testIP) {
			t.Errorf("Request %d should have been allowed, but was blocked", i+1)
		}
	}

	if limiter.Allow(testIP) {
		t.Errorf("Request 4 should have been blocked, but was allowed")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	limiter := NewIPRateLimiter(1.0, 2.0)
	testIP := "10.0.0.1"

	if !limiter.Allow(testIP) {
		t.Fatalf("First request should have been allowed")
	}

	if limiter.Allow(testIP) {
		t.Fatalf("Second request should have been blocked")
	}

	time.Sleep(600 * time.Millisecond)

	if !limiter.Allow(testIP) {
		t.Errorf("Request after waiting should have been allowed due to refill")
	}
}

func TestRateLimiter_Concurrency(t *testing.T) {
	limiter := NewIPRateLimiter(100.0, 0.0)
	testIP := "127.0.0.1"

	var wg sync.WaitGroup
	var allowedCount int
	var countMutex sync.Mutex

	for i := 0; i < 150; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if limiter.Allow(testIP) {
				countMutex.Lock()
				allowedCount++
				countMutex.Unlock()
			}
		}()
	}

	wg.Wait()

	if allowedCount != 100 {
		t.Errorf("Expected exactly 100 allowed requests, got %d", allowedCount)
	}
}
