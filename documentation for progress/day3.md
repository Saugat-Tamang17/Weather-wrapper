# Backend Development with Go — Session 03
**Project:** Weather API Wrapper Service  
**Stack:** Golang, Open-Meteo API, Docker  
**Date:** 2026-05-06

---

## What We Built

Implemented graceful shutdown with OS signal handling, containerized the service with a multi-stage Docker build, and added request middleware (request ID generation + structured logging).

---

## Concepts Covered

### 1. Graceful Shutdown — Why It Matters

Without graceful shutdown, `Ctrl+C` or `docker stop` sends `SIGINT`/`SIGTERM` and Go kills the process immediately. Any in-flight request gets a TCP reset — not a 500, not a 502, a connection drop. The client has no idea what happened.

**Graceful shutdown does three things in order:**
1. Stop accepting new connections
2. Wait for all in-flight requests to finish
3. Then exit

The timeout is critical — you don't wait forever. If a hung request exceeds the deadline, you cancel it and exit anyway.

**Why `http.ListenAndServe` had to go:**

`http.ListenAndServe` is a convenience wrapper — it creates an `http.Server` internally and you never get a reference to it. No reference means no `Shutdown`. You need to create the server explicitly:

```go
server := &http.Server{
    Addr:    ":" + cfg.Port,
    Handler: mux,
}
```

**Why the server runs in a goroutine:**

`ListenAndServe` blocks. Graceful shutdown requires something listening for OS signals concurrently. The server runs in a goroutine, `main` blocks on the signal channel.

---

### 2. `main.go` — Graceful Shutdown Implementation

```go
// Run server in goroutine — ListenAndServe blocks
go func() {
    log.Printf("Server starting on port :%s", cfg.Port)
    if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("Server failed: %v", err)
    }
}()

// Block main until shutdown signal
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

log.Println("Shutdown signal received, draining in-flight requests...")

ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

if err := server.Shutdown(ctx); err != nil {
    log.Fatalf("Forced shutdown: %v", err)
}
log.Println("Server exited cleanly")
```

**Three details that matter:**

- `http.ErrServerClosed` is not a real error — when `Shutdown` closes the listener, `ListenAndServe` returns this sentinel value. Filter it out or you'll `log.Fatalf` on a clean shutdown.
- `make(chan os.Signal, 1)` — the buffer of 1 is required. Signal delivery is async. An unbuffered channel can drop the signal if nothing is reading at the exact moment it fires.
- 15-second timeout — tuned to be above your slowest possible request (Open-Meteo timeout is 10s). After the deadline, `Shutdown` returns `context.DeadlineExceeded`.

**If there are no in-flight requests**, `Shutdown` returns immediately. The timeout is a ceiling, not a mandatory wait.

---

### 3. Docker — Multi-Stage Build

Go compiles to a single self-contained binary. The compiler is only needed at build time, not at runtime. Multi-stage builds exploit this — a heavy builder stage compiles the binary, a minimal runner stage ships only the binary.

**Stage 1 — Builder:**
```dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o weather-wrapper ./main.go
```

**Stage 2 — Runner:**
```dockerfile
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/weather-wrapper .

EXPOSE 8080

CMD ["./weather-wrapper"]
```

**Why `go.mod`/`go.sum` are copied before the rest of the source:**

Docker builds layer by layer and caches each layer. If you copy all source first and then download dependencies, any `.go` file change invalidates the download layer — re-downloading everything on every build. Copying the manifest first means dependency downloads are only re-run when `go.mod` or `go.sum` actually change. On most builds that layer is a cache hit.

**Why CA certificates are required:**

`alpine:3.21` ships with no TLS root certificates. Your service calls Open-Meteo over HTTPS. Without `ca-certificates`, every outbound request fails with:
```
x509: certificate signed by unknown authority
```

**Why `CGO_ENABLED=0 GOOS=linux`:**

You're building on Windows. `GOOS=linux` tells the compiler to target Linux regardless of the host OS. `CGO_ENABLED=0` produces a fully static binary with no C dependencies — required in a minimal Alpine environment.

**Why `alpine:latest` is wrong:**

Non-deterministic. Six months from now `latest` might pull a different Alpine version and something breaks silently. Always pin — `alpine:3.21`.

**`.dockerignore`** — place in project root to keep the build context clean:
```
.git
*.exe
*.test
tmp/
```

---

### 4. Middleware — The Pattern

Middleware is code that runs on every request, before and/or after your handler, without your handler knowing about it. Cross-cutting concerns (logging, auth, rate limiting, panic recovery) stay separate from business logic.

**In Go, middleware is a function that takes an `http.Handler` and returns an `http.Handler`:**

```go
func MyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // runs before handler
        next.ServeHTTP(w, r)
        // runs after handler
    })
}
```

`next` is whatever comes after — another middleware or the final handler. Not calling `next.ServeHTTP` stops the request there. That's how auth middleware blocks unauthorized requests.

---

### 5. `internal/middleware/middleware.go`

```go
package middleware

import (
    "context"
    "fmt"
    "log"
    "math/rand"
    "net/http"
    "time"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := fmt.Sprintf("%016x", rand.Uint64())
        ctx := context.WithValue(r.Context(), RequestIDKey, id)
        w.Header().Set("X-Request-ID", id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

type responseWriter struct {
    http.ResponseWriter
    status int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.status = code
    rw.ResponseWriter.WriteHeader(code)
}

func Logger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
        next.ServeHTTP(rw, r)
        id, _ := r.Context().Value(RequestIDKey).(string)
        log.Printf("[%s] %s %s %d %v", id, r.Method, r.URL.Path, rw.status, time.Since(start))
    })
}
```

**Wired in `main.go`:**
```go
server := &http.Server{
    Addr:    ":" + cfg.Port,
    Handler: middleware.Logger(middleware.RequestID(mux)),
}
```

**Three things that matter:**

- **`responseWriter` wrapper** — `http.ResponseWriter` doesn't expose the status code after `WriteHeader` is called. You wrap the real writer, intercept `WriteHeader`, save the code, delegate to the real writer. Standard Go pattern.
- **Middleware ordering** — `Logger(RequestID(mux))` means `RequestID` runs first. Logger logs *after* `next.ServeHTTP` returns, so the request ID is already in context by then. Reversed order means logging an empty ID.
- **Unexported `contextKey` type** — using a plain string as a context key means any package using `"request_id"` collides with yours. A custom unexported type makes collisions impossible — only this package can produce a value of that type.

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
| Panic recovery middleware | Not done |
| Tests | Not done |
| Rate limiting | Not done |
| Input validation (coordinate ranges) | Not done |

**Honest completion: ~55-60%** of a real shippable service.

---

## What We Did Not Do Yet (Next Session)

- Panic recovery middleware — one nil pointer dereference currently crashes the entire server
- Tests — unit tests for handler and weather client
- Input validation — lat/lng coordinate range checks
- Rate limiting

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
- internal/handler/handler.go: parses lat/lng query params, validates and parses to float64, calls weather client, returns JSON, 502 for upstream failures
- internal/middleware/middleware.go: RequestID middleware (generates hex ID, sets X-Request-ID header, attaches to context), Logger middleware (wraps ResponseWriter to capture status code, logs method/path/status/duration)
- Dockerfile: multi-stage build (golang:1.24-alpine builder, alpine:3.21 runner), CA certificates, CGO_ENABLED=0 GOOS=linux, layer caching via go.mod/go.sum copy order
- .dockerignore: .git, *.exe, *.test, tmp/

Next steps:
1. Panic recovery middleware — catch panics in handlers, return 500, log the stack trace, keep the server alive
2. Tests — unit tests for handler and weather client
3. Input validation — reject coordinates outside valid ranges (lat -90 to 90, lng -180 to 180)
4. Rate limiting middleware

Resume from there.
```
