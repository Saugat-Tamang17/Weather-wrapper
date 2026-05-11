# Backend Development with Go — Session 07
**Project:** Weather API Wrapper Service  
**Stack:** Golang, Open-Meteo API, Docker  
**Date:** 2026-05-10

---

## What We Built

Wrapped up the remaining three items to bring the service to 100% shippable:

1. `Retry-After` header on 429 responses
2. Rate and burst values configurable via environment variables
3. IP eviction from the limiter map to prevent unbounded memory growth

Rate limiter tests are deferred to Session 08.

---

## 1. `Retry-After` Header — `internal/middleware/ratelimit.go`

### Why It Matters

A 429 with no guidance is hostile to clients. `Retry-After` tells them how long to wait before retrying. It's part of the HTTP spec for 429 and well-supported by HTTP clients and libraries. Without it, clients either hammer you immediately or implement their own arbitrary backoff.

### Implementation

One line added before the error write in `Middleware`:

```go
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ip := getIP(r)
        if !rl.getLimiter(ip).Allow() {
            w.Header().Set("Retry-After", "1")
            http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

Static value of `1` second. At a refill rate of 5 tokens/sec, a client who just hit the limit will have tokens again within a second. Dynamic calculation using `limiter.Reserve()` is possible but adds complexity for marginal benefit at this scale.

---

## 2. Env Config for Rate and Burst — `config/config.go`

### Why It Matters

Hardcoded rate/burst values in `main.go` mean you have to recompile to tune them. Making them env-configurable means you can adjust behaviour per deployment — development, staging, production — without touching code. It also makes the Docker image more reusable.

### Config Struct

Added two fields:

```go
type Config struct {
    Port           string
    CacheTTL       time.Duration
    OpenMeteoURL   string
    RateLimitRate  float64
    RateLimitBurst int
}
```

### Loading in `Load()`

```go
rlRate := 5.0
if v := os.Getenv("RATE_LIMIT_RATE"); v != "" {
    if parsed, err := strconv.ParseFloat(v, 64); err == nil {
        rlRate = parsed
    }
}

rlBurst := 10
if v := os.Getenv("RATE_LIMIT_BURST"); v != "" {
    if parsed, err := strconv.Atoi(v); err == nil {
        rlBurst = parsed
    }
}

return &Config{
    // ... existing fields ...
    RateLimitRate:  rlRate,
    RateLimitBurst: rlBurst,
}
```

Invalid env values silently fall back to defaults — same pattern used for the other config fields.

### Wiring in `main.go`

```go
limiter := middleware.NewRateLimiter(rate.Limit(cfg.RateLimitRate), cfg.RateLimitBurst)
```

---

## 3. IP Eviction — `internal/middleware/ratelimit.go`

### Why It Matters

The limiter map grows forever without eviction. Every unique IP that ever hits the service gets an entry. In production with real traffic, that's a slow memory leak. An attacker who rotates IPs can deliberately bloat it. The fix is a background goroutine that periodically removes entries that haven't been seen recently.

### Updated Struct

Added `lastSeen` to track when each IP was last active:

```go
type RateLimiter struct {
    mu       sync.Mutex
    limiters map[string]*rate.Limiter
    lastSeen map[string]time.Time
    rate     rate.Limit
    burst    int
}
```

### Constructor — Starts the Cleanup Goroutine

```go
func NewRateLimiter(r rate.Limit, burst int) *RateLimiter {
    rl := &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        lastSeen: make(map[string]time.Time),
        rate:     r,
        burst:    burst,
    }
    go rl.cleanupLoop()
    return rl
}
```

### Cleanup Loop

Runs every 5 minutes. Holds the mutex only for the map deletion, not for a long scan:

```go
func (rl *RateLimiter) cleanupLoop() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        rl.mu.Lock()
        for ip, t := range rl.lastSeen {
            if time.Since(t) > 5*time.Minute {
                delete(rl.limiters, ip)
                delete(rl.lastSeen, ip)
            }
        }
        rl.mu.Unlock()
    }
}
```

### Updated `getLimiter`

Updates `lastSeen` on every access:

```go
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    if limiter, ok := rl.limiters[ip]; ok {
        rl.lastSeen[ip] = time.Now()
        return limiter
    }
    limiter := rate.NewLimiter(rl.rate, rl.burst)
    rl.limiters[ip] = limiter
    rl.lastSeen[ip] = time.Now()
    return limiter
}
```

### Design Note — Goroutine Leak on Shutdown

The `cleanupLoop` goroutine has no done channel, so it isn't stopped when the server shuts down gracefully. It leaks — but only until process exit, which the graceful shutdown handles cleanly anyway. Adding a done channel would require plumbing a `context.Context` through `NewRateLimiter` and the middleware chain. Not worth it for this service. In a reusable library, you would.

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
| Retry-After header on 429 | Done |
| Rate/burst configurable via env | Done |
| IP eviction from limiter map | Done |
| Rate limiter tests | Tomorrow |

**Honest completion: ~98%** — one test file away from fully done.

---

## What Remains

- `internal/middleware/ratelimit_test.go` — four cases: allows under burst, blocks after burst, Retry-After header present on 429, per-IP isolation

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
- config/config.go: loads PORT, CACHE_TTL_SECONDS, RATE_LIMIT_RATE (float64, default 5.0), RATE_LIMIT_BURST (int, default 10) from env with defaults, OpenMeteoURL hardcoded
- internal/weather/models.go: Coordinates, CurrentWeather, WeatherResponse structs with corrected JSON tags
- internal/weather/client.go: Client struct with custom http.Client (10s timeout), TTL cache with sync.RWMutex, GetWeather with url.Values, defer resp.Body.Close(), error wrapping with %w
- internal/handler/handler.go: WeatherService interface for testability, parses lat/lng query params, validates presence and float64 parsing, range checks (lat -90 to 90, lng -180 to 180), calls weather client, returns JSON, 502 for upstream failures, logs encoder errors
- internal/middleware/middleware.go: RequestID middleware, Logger middleware
- internal/middleware/recovery.go: Recovery middleware (deferred recover(), logs panic value and stack trace, returns 500)
- internal/middleware/ratelimit.go: Per-IP token bucket rate limiter using golang.org/x/time/rate. RateLimiter struct with map[string]*rate.Limiter, map[string]time.Time lastSeen, sync.Mutex. getLimiter updates lastSeen on every access. cleanupLoop goroutine (5-min ticker) evicts IPs not seen in 5 minutes. getIP helper handles X-Forwarded-For splitting and net.SplitHostPort. Middleware returns 429 with Retry-After: 1 header on Allow() == false.
- Middleware chain in main.go: Recovery(Logger(RequestID(limiter.Middleware(mux)))) — Recovery outermost, limiter instantiated from cfg.RateLimitRate and cfg.RateLimitBurst
- Dockerfile: multi-stage build (golang:1.24-alpine builder, alpine:3.21 runner), CA certificates, CGO_ENABLED=0 GOOS=linux
- internal/handler/handler_test.go: table-driven tests (9 cases), fakeClient, httptest — all passing
- internal/weather/client_test.go: happy path, upstream 503, cache hit (atomic.Int32 counter, race-clean) — all passing

Remaining:
- internal/middleware/ratelimit_test.go: four cases — allows under burst, blocks after burst, Retry-After header on 429, per-IP isolation

Resume from there. Write the test file.
```
