# Backend Development with Go — Session 05
**Project:** Weather API Wrapper Service  
**Stack:** Golang, Open-Meteo API, Docker  
**Date:** 2026-05-09

---

## What We Built

Completed the weather client unit tests using `httptest.NewServer` — a local in-process fake HTTP server that the client hits instead of the real Open-Meteo API. No network, fully controlled, fast.

---

## Why Client Tests Are Different From Handler Tests

The handler tests were straightforward — you injected a `fakeClient` through the `WeatherService` interface. No HTTP involved at all.

The weather client is different. It actually makes HTTP calls. You can't mock that with an interface — you need a real HTTP server to respond to. `httptest.NewServer` gives you exactly that: a real server running locally, with a handler you control entirely.

---

## Confirming the Client Was Already Injectable

Before writing tests, the first step was checking how the client receives its base URL. It was already a field on the struct and accepted via `NewClient`:

```go
func NewClient(baseURL string, cacheTTLSeconds int) *Client {
    return &Client{
        baseURL: baseURL,
        ...
    }
}
```

No refactor needed. In production, `main.go` passes the real Open-Meteo URL. In tests, you pass `server.URL` from `httptest.NewServer`. Same client, different target.

---

## Test Cases Covered

### 1. Happy Path

Fake server returns valid JSON with latitude, longitude, and current weather. Assert no error, assert response fields match.

```go
func TestClient_GetWeather_HappyPath(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{
            "latitude": 27.7,
            "longitude": 85.3,
            "current": {
                "temperature_2m": 22.5
            }
        }`))
    }))
    defer server.Close()

    client := NewClient(server.URL, 60)
    resp, err := client.GetWeather(Coordinates{Latitude: 27.7, Longitude: 85.3})

    if err != nil {
        t.Fatalf("Expected no error, got: %v", err)
    }
    if resp.Latitude != 27.7 {
        t.Errorf("latitude mismatch: got %v", resp.Latitude)
    }
    if resp.Longitude != 85.3 {
        t.Errorf("longitude mismatch: got %v", resp.Longitude)
    }
}
```

**Note on float64 equality:** Comparing `float64` with `!=` works here in practice, but floating point equality is technically unreliable. Fine for coordinates like these, just something to be aware of.

---

### 2. Upstream Non-200

Fake server returns 503. Assert `err != nil`, `resp == nil`, and error message contains `"upstream returned 503"`.

```go
func TestClient_GetWeather_Upstream503(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusServiceUnavailable)
    }))
    defer server.Close()

    client := NewClient(server.URL, 60)
    resp, err := client.GetWeather(Coordinates{Latitude: 27.7, Longitude: 85.3})

    if err == nil {
        t.Fatalf("expected error, got nil")
    }
    if resp != nil {
        t.Fatalf("expected nil response, got %+v", resp)
    }
    if !strings.Contains(err.Error(), "upstream returned 503") {
        t.Fatalf("unexpected error message: %v", err)
    }
}
```

---

### 3. Cache Hit

Call `GetWeather` twice with the same coordinates. The fake server tracks how many times it was hit using an `atomic.Int32` counter. Assert the server received exactly 1 request — the second call must have been served from cache.

```go
func TestClient_GetWeather_CacheHit(t *testing.T) {
    var callCount atomic.Int32

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        callCount.Add(1)
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{
            "latitude": 27.7,
            "longitude": 85.3,
            "current": {"temperature_2m": 22.5}
        }`))
    }))
    defer server.Close()

    client := NewClient(server.URL, 60)
    coords := Coordinates{Latitude: 27.7, Longitude: 85.3}

    _, err := client.GetWeather(coords)
    if err != nil {
        t.Fatalf("first call failed: %v", err)
    }
    _, err = client.GetWeather(coords)
    if err != nil {
        t.Fatalf("second call failed: %v", err)
    }
    if callCount.Load() != 1 {
        t.Fatalf("expected 1 upstream call, got %d", callCount.Load())
    }
}
```

---

## The Race Condition Caught

The first version of the cache test used a plain `int` for `callCount`. That's a data race — the HTTP handler runs in its own goroutine and writes to `callCount` while the test goroutine reads it. The race detector catches this:

```bash
go test ./internal/weather/ -race -v
```

Fix: use `sync/atomic.Int32`. Reads and writes are atomic by definition — no mutex needed, no race.

**Rule:** any variable shared between a test goroutine and an HTTP handler goroutine must be synchronized. The handler is not running in your goroutine.

---

## All Tests Passing Clean

```
=== RUN   TestClient_GetWeather_HappyPath
--- PASS: TestClient_GetWeather_HappyPath (0.01s)
=== RUN   TestClient_GetWeather_Upstream503
--- PASS: TestClient_GetWeather_Upstream503 (0.00s)
=== RUN   TestClient_GetWeather_CacheHit
--- PASS: TestClient_GetWeather_CacheHit (0.00s)
PASS
ok      github.com/Saugat-Tamang17/weather-wrapper/internal/weather     2.755s
```

No races. No network. No flakiness.

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
| Rate limiting | Not done |

---

## Next Session

Rate limiting middleware — token bucket algorithm using `golang.org/x/time/rate`. Per-IP limiting with a map of IP → limiter. Wire into the middleware chain.
