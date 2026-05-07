# Backend Development with Go — Session 04
**Project:** Weather API Wrapper Service  
**Stack:** Golang, Open-Meteo API, Docker  
**Date:** 2026-05-07

---

## What We Built

Added panic recovery middleware, input validation for coordinate ranges, and unit tests for the HTTP handler using table-driven tests and interface-based dependency injection.

---

## Concepts Covered

### 1. Panics vs Errors — They Are Not the Same

`if err != nil` handles **errors** — values that functions explicitly return. Your code controls this flow entirely.

A **panic** is different. It's not a return value. It's the runtime saying "something went catastrophically wrong" and unwinding the call stack immediately. Common causes: nil pointer dereference, index out of bounds, wrong type assertion. Your function never gets a chance to return anything — execution blows up.

If a panic is not caught, it unwinds all the way up and kills the goroutine. Since HTTP handlers run in goroutines spun up by the server, an uncaught panic in a handler **crashes the entire server process**. Every user hits dead air.

Go gives you two tools:

- `recover()` — call inside a deferred function. Catches the panic, stops the unwind, returns whatever value was passed to `panic()`. Outside a `defer`, it does nothing.
- `debug.Stack()` from `runtime/debug` — returns the current goroutine's stack trace as `[]byte`. Log this so you know exactly where the panic originated.

**Important:** `defer` always runs — panic or not. When there's no panic, `recover()` returns `nil` and the `if` block is skipped. It is not "only runs if panic detected."

---

### 2. `internal/middleware/recovery.go`

```go
package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic Recovered:%v\n%s", err, debug.Stack())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
```

**Wired in `main.go`:**
```go
server := &http.Server{
    Addr:    ":" + cfg.Port,
    Handler: middleware.Recovery(middleware.Logger(middleware.RequestID(mux))),
}
```

**Why Recovery must be outermost:**

Recovery catches panics from everything **inside** it. If it were innermost, a panic in Logger or RequestID would never reach it. Outermost means it wraps the entire chain — a panic anywhere below gets caught.

Execution order for a request:
```
Recovery → Logger → RequestID → mux/handler
```

---

### 3. Input Validation — Coordinate Range Checks

Added after `ParseFloat` succeeds, before the client call. Lat must be -90 to 90, lng -180 to 180.

```go
if lat < -90 || lat > 90 {
    http.Error(w, "latitude must be between -90 and 90", http.StatusBadRequest)
    return
}

if long < -180 || long > 180 {
    http.Error(w, "longitude must be between -180 and 180", http.StatusBadRequest)
    return
}
```

Also added proper error handling for the JSON encoder — you can't write an HTTP error after the response body has started, but you should still log it:

```go
if err := json.NewEncoder(w).Encode(result); err != nil {
    log.Printf("failed to encode response: %v", err)
}
```

---

### 4. Interfaces for Testability — Dependency Injection

The handler originally depended on `*weather.Client` directly — a concrete type. You can't swap it out for a fake in tests.

The fix: define an **interface** with only the method your handler needs:

```go
type WeatherService interface {
    GetWeather(coords weather.Coordinates) (*weather.WeatherResponse, error)
}

type WeatherHandler struct {
    client WeatherService
}

func New(client WeatherService) *WeatherHandler {
    return &WeatherHandler{client: client}
}
```

`*weather.Client` already has `GetWeather` with the right signature — it satisfies the interface automatically. No changes needed in the client. But now in tests you can pass in a fake implementation instead of the real client.

This is **dependency injection** — you inject the dependency through the interface rather than hardcoding the concrete type. Standard production Go pattern.

---

### 5. Unit Tests — `internal/handler/handler_test.go`

**Key packages:**
- `testing` — Go's built-in test package
- `net/http/httptest` — fake request and response recorder, no network needed
  - `httptest.NewRequest(method, url, body)` — constructs a fake `*http.Request`
  - `httptest.NewRecorder()` — fake `http.ResponseWriter` that records status, headers, body

**The fake client:**
```go
type fakeClient struct {
    response *weather.WeatherResponse
    err      error
}

func (f *fakeClient) GetWeather(coords weather.Coordinates) (*weather.WeatherResponse, error) {
    return f.response, f.err
}
```

Set `response` for success cases, set `err` for the 502 case. No network, fully controlled.

**Table-driven tests — standard Go convention:**

```go
func TestWeatherHandler(t *testing.T) {
    tests := []struct {
        name       string
        url        string
        fakeResp   *weather.WeatherResponse
        fakeErr    error
        wantStatus int
    }{
        {name: "missing params", url: "/weather", wantStatus: 400},
        {name: "Invalid Lat", url: "/weather?lat=abc&lng=10", wantStatus: 400},
        {name: "Invalid Long", url: "/weather?lat=57&lng=abc", wantStatus: 400},
        {name: "lat value too low", url: "/weather?lat=-999&lng=10", wantStatus: 400},
        {name: "lat value too high", url: "/weather?lat=999&lng=10", wantStatus: 400},
        {name: "long value too low", url: "/weather?lat=10&lng=-999", wantStatus: 400},
        {name: "long value too high", url: "/weather?lat=10&lng=999", wantStatus: 400},
        {name: "service Failure", url: "/weather?lat=10&lng=10", fakeErr: errors.New("api down"), wantStatus: 502},
        {name: "happy path", url: "/weather?lat=10&lng=20", fakeResp: &weather.WeatherResponse{}, wantStatus: 200},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            fake := &fakeClient{response: tt.fakeResp, err: tt.fakeErr}
            h := New(fake)
            req := httptest.NewRequest("GET", tt.url, nil)
            rec := httptest.NewRecorder()
            h.ServeHTTP(rec, req)
            if rec.Code != tt.wantStatus {
                t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
            }
        })
    }
}
```

All 9 cases pass. No network. No server.

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
| Weather client unit tests | Not done |
| Rate limiting | Not done |

**Honest completion: ~80%** of a real shippable service.

---

## What We Did Not Do Yet (Next Session)

- Weather client tests — approach is clear: `httptest.NewServer` spins up a local HTTP server, point the client at it instead of Open-Meteo. Need to check how the client receives its URL first.
- Rate limiting middleware

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
- internal/middleware/middleware.go: RequestID middleware (generates hex ID, sets X-Request-ID header, attaches to context with unexported contextKey type), Logger middleware (wraps ResponseWriter to capture status code, logs method/path/status/duration)
- internal/middleware/recovery.go: Recovery middleware (deferred recover(), logs panic value and stack trace, returns 500)
- Middleware chain in main.go: Recovery(Logger(RequestID(mux))) — Recovery outermost
- Dockerfile: multi-stage build (golang:1.24-alpine builder, alpine:3.21 runner), CA certificates, CGO_ENABLED=0 GOOS=linux, layer caching via go.mod/go.sum copy order
- .dockerignore: .git, *.exe, *.test, tmp/
- internal/handler/handler_test.go: table-driven tests (9 cases), fakeClient implementing WeatherService interface, httptest.NewRequest + httptest.NewRecorder, all passing

Next steps:
1. Weather client tests — use httptest.NewServer to spin up a local fake HTTP server, point the client at it instead of Open-Meteo. First check how the client currently receives its base URL.
2. Rate limiting middleware

Resume from there.
```
