package limiter

import (
	"fmt"
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

func TestRateLimiter_RaceCondition(t *testing.T) {
	limiter := NewIPRateLimiter(2000.0, 10.0)
	targetIP := "192.168.1.1"

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			limiter.Allow(targetIP)
		}()
	}

	wg.Wait()
}

// Benchmark 1: The Localhost Trap (All traffic hits exactly ONE shard)
func BenchmarkRateLimiter_SingleIP(b *testing.B) {
	limiter := NewIPRateLimiter(1000.0, 10.0)
	targetIP := "127.0.0.1"

	b.ResetTimer() // Reset timer so setup doesn't count against our score

	// RunParallel executes the loop concurrently across all your CPU cores
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow(targetIP)
		}
	})
}

func BenchmarkRateLimiter_MultiIP(b *testing.B) {
	limiter := NewIPRateLimiter(1000.0, 10.0)

	// Pre-generate 10,000 distinct, fake IP addresses so string allocation
	// doesn't slow down our benchmark math
	ips := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		ips[i] = fmt.Sprintf("10.0.%d.%d", i/256, i%256)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Cycle through the 10,000 different IPs
			limiter.Allow(ips[i%10000])
			i++
		}
	})
}
