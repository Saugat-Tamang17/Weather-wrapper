# Backend Development with Go — Session 06
**Project:** Weather API Wrapper Service  
**Stack:** Golang, Open-Meteo API, Docker  
**Date:** 2026-05-09

---

## What We Built

Implemented a per-IP token bucket rate limiter as middleware using `golang.org/x/time/rate`, wired it into the middleware chain in `main.go`.

---

## Theory — Everything You Need to Know About Rate Limiting

### Why It Exists

Without rate limiting, a single client can hammer your service with thousands of requests per second. That exhausts server resources, starves legitimate users, and if your service calls a third party (like Open-Meteo), you risk getting banned upstream. Rate limiting caps how much any single client can consume.

---

### The Three Main Algorithms

**1. Fixed Window**

Divide time into fixed buckets — say 1 minute windows. Each client gets 100 requests per window. Simple, but has a burst problem: a client can send 100 requests at 12:00:59 and another 100 at 12:01:00 — 200 requests in 2 seconds, which defeats the purpose.

**2. Sliding Window**

Instead of fixed buckets, look at the last N seconds from *now* continuously. Solves the burst problem at window boundaries. More accurate but costs more memory — you need to store timestamps of recent requests per client.

**3. Token Bucket**

Industry standard. Each client has a bucket with a max capacity of N tokens. Tokens refill at a fixed rate (e.g. 5 per second). Each request costs 1 token. If the bucket is empty, the request is rejected with 429.

Key property: naturally allows short bursts (up to bucket capacity) while enforcing a sustained average rate. A user refreshing a page quickly shouldn't be blocked immediately — but sustained abuse gets cut off.

---

### Token Bucket Mechanics

Two numbers define it:
- **Rate** — how fast tokens refill (e.g. 5 per second)
- **Burst** — max bucket capacity (e.g. 10)

A client who hasn't made requests in a while accumulates tokens up to the burst limit. They can spend those quickly, then get throttled to the refill rate. A client hammering the service drains the bucket and starts getting 429s.

You don't store tokens discretely. You store:
- The last time the bucket was checked
- How many tokens were in it at that point

When a request comes in, you calculate tokens accumulated since last check (`elapsed * rate`), cap at burst, subtract 1. This is **lazy evaluation** — compute current state on demand rather than running a background ticker.

---

### Per-IP vs Global

Global limits total traffic to your service. Per-IP limits each individual client. You almost always want per-IP — one bad actor exhausts a global limit and blocks everyone else.

Per-IP means a map of `IP → limiter state`. In production you'd add eviction (remove IPs not seen in X minutes) to prevent unbounded memory growth. Skipped here for simplicity.

---

### What Happens on Rejection

Return **HTTP 429 Too Many Requests**. Optionally set a `Retry-After` header telling the client how long to wait. This service returns 429 with a plain text body.

---

### Where It Sits in the Middleware Chain

```
Recovery → Logger → RequestID → RateLimit → mux/handler
```

Recovery stays outermost — always. Logger wraps RateLimit so 429 responses get logged. RequestID runs before RateLimit so the ID is attached before limiting decisions are made.

---

## Implementation — `internal/middleware/ratelimit.go`

```go
package middleware

import (
    "net"
    "net/http"
    "strings"
    "sync"

    "golang.org/x/time/rate"
)

type RateLimiter struct {
    mu       sync.Mutex
    limiters map[string]*rate.Limiter
    rate     rate.Limit
    burst    int
}

func NewRateLimiter(r rate.Limit, burst int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        rate:     r,
        burst:    burst,
    }
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    if limiter, ok := rl.limiters[ip]; ok {
        return limiter
    }
    limiter := rate.NewLimiter(rl.rate, rl.burst)
    rl.limiters[ip] = limiter
    return limiter
}

func getIP(r *http.Request) string {
    ip := r.Header.Get("X-Forwarded-For")
    if ip != "" {
        parts := strings.Split(ip, ",")
        return strings.TrimSpace(parts[0])
    }
    host, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil {
        return r.RemoteAddr
    }
    return host
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ip := getIP(r)
        if !rl.getLimiter(ip).Allow() {
            http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

---

## Three Design Decisions Worth Understanding

### 1. Why `getLimiter` uses `Lock` not `RLock`

`getLimiter` both reads and writes the map — it checks if a limiter exists, and if not, creates and stores one. You can't hold an `RLock` and write. You'd need to release it and acquire a full `Lock`, which opens a race window between the two operations. Simpler and correct to just take the full `Lock` upfront — the critical section is tiny.

### 2. Why `r.RemoteAddr` alone is wrong

`RemoteAddr` returns `"host:port"` — not just the IP. So `"192.168.1.1:54321"` and `"192.168.1.1:54890"` are different map keys even though it's the same client. Every new TCP connection gets a new port, so the same user would bypass the per-IP limiter entirely. Fix: `net.SplitHostPort` to extract just the host.

### 3. Why `X-Forwarded-For` needs splitting

`X-Forwarded-For` can contain multiple IPs comma-separated — client first, then each proxy in the chain:

```
X-Forwarded-For: 203.0.113.5, 70.41.3.18, 150.172.238.178
```

Using the full string as a map key means the same client looks different depending on the proxy chain. Take only `parts[0]` — the real client IP.

**Caveat:** `X-Forwarded-For` is trivially spoofable. A client can set it themselves and appear to come from any IP. In production you'd only trust it if the request is coming from a known proxy. Fine for this service, but know the limitation.

---

## Wiring in `main.go`

```go
limiter := middleware.NewRateLimiter(5, 10) // 5 tokens/sec, burst of 10

server := &http.Server{
    Addr:    ":" + cfg.Port,
    Handler: middleware.Recovery(middleware.Logger(middleware.RequestID(limiter.Middleware(mux)))),
}
```

5 tokens per second, burst of 10. A fresh client can make 10 rapid requests, then gets throttled to 5 per second. Sustained abuse hits 429.

---

## Verified Working

After the burst capacity drains, requests return 429:

```
429
429
429
429
```

After waiting for the bucket to refill (~2 seconds at 5/sec), 200s resume before hitting 429 again — confirming the token bucket is refilling correctly.

---

## Project Status

| Area | Status |
|---|---|
| Project structure | Done |
| HTTP handler + validation | Done |
| TTL cache with RWMutex | Done |
| Graceful shutdown | Done |
| Docker multi-stage build | Done |
| Request ID + logging middleware | Done |
| Panic recovery middleware | Done |
| Input validation (coordinate ranges) | Done |
| Handler unit tests | Done |
| Weather client unit tests | Done |
| Rate limiting | Done |

**Honest completion: ~95%** of a real shippable service.

---

## What Remains

- Rate limiter tests
- IP limiter eviction (remove stale IPs from the map to prevent unbounded memory growth)
- `Retry-After` header on 429 responses
- Making rate and burst values configurable via environment variables

---

## Summary Prompt (Paste This at the Start of Next Session)

```
SYSTEM PROMPT (Senior Backend Engineer Mode)
You are a senior backend software engineer with 10+ years of industry experience working in production systems. Your role is to teach me backend development the way it is taught in real software teams, not like a tutorial blog. Be direct, honest, and practical. Correct me clearly when I'm wrong. Focus on how things actually work in production. No diagrams, no excessive formatting, no filler. Everything in Golang.

SESSION CONTEXT:
We are building a weather API wrapper service in Go that calls Open-Meteo (free, no API key).

Completed so far:
- go mod init, project folder structure (internal/, config/, handler/, weather/, middleware/)
- main.go: http.NewServeMux, /health endpoint, /weather route, explicit http.Server, graceful shutdown with SIGINT/SIGTERM handling, 15s timeout context
- config/config.go: loads PORT and CACHE_TTL_SECONDS from env with defaults, OpenMeteoURL hardcoded
- internal/weather/models.go: Coordinates, CurrentWeather, WeatherResponse structs with corrected JSON tags
- internal/weather/client.go: Client struct with custom http.Client (10s timeout), TTL cache with sync.RWMutex, GetWeather with url.Values, defer resp.Body.Close(), error wrapping with %w
- internal/handler/handler.go: WeatherService interface for testability, parses lat/lng query params, validates presence and float64 parsing, range checks (lat -90 to 90, lng -180 to 180), calls weather client, returns JSON, 502 for upstream failures, logs encoder errors
- internal/middleware/middleware.go: RequestID middleware, Logger middleware
- internal/middleware/recovery.go: Recovery middleware (deferred recover(), logs panic value and stack trace, returns 500)
- internal/middleware/ratelimit.go: Per-IP token bucket rate limiter using golang.org/x/time/rate. RateLimiter struct with map[string]*rate.Limiter, sync.Mutex, getLimiter method, getIP helper (handles X-Forwarded-For splitting and net.SplitHostPort), Middleware method returning 429 on Allow() == false
- Middleware chain in main.go: Recovery(Logger(RequestID(limiter.Middleware(mux)))) — Recovery outermost, limiter instantiated with rate=5, burst=10
- Dockerfile: multi-stage build (golang:1.24-alpine builder, alpine:3.21 runner), CA certificates, CGO_ENABLED=0 GOOS=linux
- internal/handler/handler_test.go: table-driven tests (9 cases), fakeClient, httptest — all passing
- internal/weather/client_test.go: happy path, upstream 503, cache hit (atomic.Int32 counter, race-clean) — all passing

Remaining:
- Rate limiter tests
- IP eviction from limiter map
- Retry-After header on 429
- Rate/burst configurable via env

Resume from there.
```
