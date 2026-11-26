package collect

import (
	"crypto/sha256"
	"sync"
	"time"
)

// Limiter is a per-client token bucket kept only in memory. Clients are
// keyed by a salted hash of their address; the raw IP is never stored,
// and the salt changes every restart.
type Limiter struct {
	mu      sync.Mutex
	rate    float64 // tokens per second
	burst   float64
	buckets map[[16]byte]*bucket
	salt    [16]byte
	last    time.Time
}

type bucket struct {
	tokens float64
	at     time.Time
}

func NewLimiter(perMinute, burst int) *Limiter {
	l := &Limiter{rate: float64(perMinute) / 60, burst: float64(burst), buckets: map[[16]byte]*bucket{}}
	now := time.Now().UnixNano()
	for i := range l.salt {
		l.salt[i] = byte(now >> (i % 8 * 8))
	}
	return l
}

func (l *Limiter) key(client string) [16]byte {
	h := sha256.Sum256(append(l.salt[:], client...))
	var k [16]byte
	copy(k[:], h[:16])
	return k
}

// Allow consumes one token for client.
func (l *Limiter) Allow(client string) bool {
	now := time.Now()
	k := l.key(client)
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.last) > 10*time.Minute {
		for kk, b := range l.buckets {
			if now.Sub(b.at) > 10*time.Minute {
				delete(l.buckets, kk)
			}
		}
		l.last = now
	}
	b := l.buckets[k]
	if b == nil {
		b = &bucket{tokens: l.burst, at: now}
		l.buckets[k] = b
	}
	b.tokens += now.Sub(b.at).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.at = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
