package readapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/httpx"
)

// Route classes of the rate limiter. The published table gives each class
// as a range (api/MACHINE-API.md § Rate limits); the exact values are
// environment configuration.
const (
	classText      = "text"      // official version text, sections, lines, policy versions
	classLive      = "live"      // catalog, bills, explanations, derived documents, policy, log
	classBulk      = "bulk"      // feed and exports
	classDiscovery = "discovery" // /v1 and the spec
)

// Default limits in requests per minute per client per class. They sit inside
// the published ranges; operators set the real values through configuration.
const (
	defaultText      = 600
	defaultLive      = 300
	defaultBulk      = 120
	defaultDiscovery = 600
)

const (
	maxBuckets    = 100_000
	maxRetryAfter = 60
	secondsPerMin = 60.0
	keyBytes      = 32
	burstSeconds  = 10.0 // a bucket holds this many seconds of its rate
)

// RateLimits maps a route class to requests per minute; a class with a value
// of zero or less is not limited, and a class missing from the map takes its
// default.
type RateLimits map[string]int

// DefaultRateLimits returns the built-in limits.
func DefaultRateLimits() RateLimits {
	return RateLimits{
		classText: defaultText, classLive: defaultLive, classBulk: defaultBulk, classDiscovery: defaultDiscovery,
	}
}

// ParseRateLimits reads "text=600,live=300" over the defaults. An unknown
// class or a bad number is ErrInvalid.
func ParseRateLimits(spec string) (RateLimits, error) {
	out := DefaultRateLimits()
	if strings.TrimSpace(spec) == "" {
		return out, nil
	}
	for part := range strings.SplitSeq(spec, ",") {
		name, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if _, known := out[name]; !ok || !known {
			return nil, fmt.Errorf("rate limit %q: %w", part, data.ErrInvalid)
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("rate limit %q: %w", part, data.ErrInvalid)
		}
		out[name] = n
	}
	return out, nil
}

// classify returns the rate class of a request path.
func classify(path string) string {
	switch {
	case path == "/v1" || path == specPath:
		return classDiscovery
	case path == "/v1/feed" || strings.HasPrefix(path, "/v1/exports"):
		return classBulk
	case strings.HasPrefix(path, "/v1/selection-policy/"):
		return classText
	case strings.HasPrefix(path, pathPrefix) && (strings.Contains(path, "/s/") || isVersionRoot(path)):
		return classText
	}
	return classLive
}

// isVersionRoot reports whether path is /v1/bills/{bill}/v/{n}.
func isVersionRoot(path string) bool {
	rest := strings.TrimPrefix(path, pathPrefix)
	parts := strings.Split(rest, "/")
	const versionRootParts = 3
	return len(parts) == versionRootParts && parts[1] == "v"
}

type bucket struct {
	tokens float64
	last   time.Time
}

type bucketKey struct {
	client [sha256.Size]byte
	class  string
}

// limiter is an in-memory token-bucket limiter. A client is the HMAC of its
// address and the day under a key that exists only in this process, so no
// raw address is held and the identity changes every day. Limits are per
// instance, hence approximate across instances. Exceeding a limit delays the
// client by Retry-After; nothing is blocked beyond the refill time.
type limiter struct {
	limits RateLimits
	header string
	now    func() time.Time
	// debug logs each request's address shape (logShape) to log.
	debug bool
	log   *slog.Logger

	mu      sync.Mutex
	key     []byte
	buckets map[bucketKey]*bucket
}

// newLimiter builds a limiter with a fresh random key. header names a
// request header whose last comma-separated value is the client address
// (for example behind a trusted proxy); empty means the socket address.
func newLimiter(limits RateLimits, header string, now func() time.Time) (*limiter, error) {
	key := make([]byte, keyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("rate key: %w", err)
	}
	return &limiter{limits: limits, header: header, now: now, key: key, buckets: map[bucketKey]*bucket{}}, nil
}

// clientAddr extracts the client address without keeping it.
func (l *limiter) clientAddr(r *http.Request) string {
	if l.header != "" {
		if v := r.Header.Get(l.header); v != "" {
			parts := strings.Split(v, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (l *limiter) clientKey(addr string, now time.Time) [sha256.Size]byte {
	mac := hmac.New(sha256.New, l.key)
	mac.Write([]byte(addr + "|" + now.UTC().Format(time.DateOnly)))
	var out [sha256.Size]byte
	copy(out[:], mac.Sum(nil))
	return out
}

// allow takes one token; when none is left it returns the seconds to wait.
func (l *limiter) allow(r *http.Request) (ok bool, retryAfter int) {
	class := classify(r.URL.Path)
	perMin := l.limits[class]
	if perMin <= 0 {
		return true, 0
	}
	now := l.now()
	k := bucketKey{client: l.clientKey(l.clientAddr(r), now), class: class}
	rate := float64(perMin) / secondsPerMin
	capacity := math.Max(1, rate*burstSeconds)

	l.mu.Lock()
	defer l.mu.Unlock()
	b, found := l.buckets[k]
	if !found {
		if len(l.buckets) >= maxBuckets {
			l.sweep(now)
		}
		b = &bucket{tokens: capacity, last: now}
		l.buckets[k] = b
	}
	b.tokens = math.Min(capacity, b.tokens+now.Sub(b.last).Seconds()*rate)
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := int(math.Ceil((1 - b.tokens) / rate))
	return false, min(max(wait, 1), maxRetryAfter)
}

// sweep drops buckets idle for a minute; when the map is still full it
// starts over, which only ever grants clients fresh allowance.
func (l *limiter) sweep(now time.Time) {
	for k, b := range l.buckets {
		if now.Sub(b.last) > time.Minute {
			delete(l.buckets, k)
		}
	}
	if len(l.buckets) >= maxBuckets {
		l.buckets = map[bucketKey]*bucket{}
	}
}

// middleware answers 429 with Retry-After when a client is over its limit.
func (l *limiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if l.debug {
			l.logShape(r)
		}
		if ok, wait := l.allow(r); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(wait))
			w.Header().Set("Cache-Control", cacheControlNoStore)
			httpx.WriteProblem(w, http.StatusTooManyRequests, "", "slow down and retry after the interval in Retry-After")
			return
		}
		next.ServeHTTP(w, r)
	})
}
