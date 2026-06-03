package limiter

import (
	"hash/fnv"
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

// Shard contains a subset of the map and its own dedicated lock
type Shard struct {
	mu      sync.Mutex
	buckets map[string]*TokenBucket
}

type IPRateLimiter struct {
	shards     [32]*Shard // Array of 32 independent locks
	capacity   float64
	refillRate float64
}

func NewIPRateLimiter(capacity, refillRate float64) *IPRateLimiter {
	limiter := &IPRateLimiter{
		capacity:   capacity,
		refillRate: refillRate,
	}

	// Initialize all 32 shards independently
	for i := 0; i < 32; i++ {
		limiter.shards[i] = &Shard{
			buckets: make(map[string]*TokenBucket),
		}
	}
	return limiter
}

// getShardIndex hashes the IP string into a number between 0 and 31
func getShardIndex(ip string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(ip))
	return h.Sum32() % 32
}

func (limiter *IPRateLimiter) Allow(ip string) bool {
	// Find the exact shard for this IP
	shardIndex := getShardIndex(ip)
	shard := limiter.shards[shardIndex]

	// Lock ONLY this shard; the other 31 remain perfectly open
	shard.mu.Lock()
	defer shard.mu.Unlock()

	now := time.Now()
	bucket, exists := shard.buckets[ip]
	if !exists {
		shard.buckets[ip] = &TokenBucket{
			capacity:   limiter.capacity,
			tokens:     limiter.capacity - 1.0,
			refillRate: limiter.refillRate,
			lastTick:   now,
		}
		return true
	}

	elapsed := now.Sub(bucket.lastTick).Seconds()
	bucket.tokens += elapsed * limiter.refillRate

	if bucket.tokens > limiter.capacity {
		bucket.tokens = limiter.capacity
	}

	bucket.lastTick = now

	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true
	}
	return false
}
