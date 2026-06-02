package limiter

import (
	"sync"
	"time"
)

type Limiter interface {
	Allow(ip string) bool
}

type TokenBucket struct {
	capacity   float64
	tokens     float64
	refillRate float64
	lastTick   time.Time
}

type IPRateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*TokenBucket
	capacity   float64
	refillRate float64
}

func NewIPRateLimiter(capacity, refillRate float64) *IPRateLimiter {
	return &IPRateLimiter{
		buckets:    make(map[string]*TokenBucket),
		capacity:   capacity,
		refillRate: refillRate,
	}
}

func (limiter *IPRateLimiter) Allow(ip string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()
	bucket, exists := limiter.buckets[ip]
	if !exists {
		limiter.buckets[ip] = &TokenBucket{
			capacity:   limiter.capacity,
			tokens:     limiter.capacity - 1.0,
			refillRate: limiter.refillRate,
			lastTick:   now,
		}
		return true
	}

	elapsed := now.Sub(bucket.lastTick).Seconds()

	bucket.tokens += elapsed * bucket.refillRate

	if bucket.tokens > bucket.capacity {
		bucket.tokens = bucket.capacity
	}

	bucket.lastTick = now

	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true
	}
	return false
}
