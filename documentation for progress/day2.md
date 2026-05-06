# Backend Development with Go — Session 02
**Project:** Weather API Wrapper Service  
**Stack:** Golang, Open-Meteo API  
**Date:** 2026-05-03

---

## What We Built

Completed the `/weather` HTTP handler, fixed a silent API decoding bug, and implemented a thread-safe in-memory TTL cache on the weather client.

---

## Concepts Covered

### 1. HTTP Input Design — Query Params vs Path Params vs Body

The `/weather` endpoint accepts `lat` and `lng` as query parameters.

**Why query params:**
- Latitude and longitude are *filter inputs* to a computation, not identifiers for a stored resource.
- Path params (`/weather/27.71/85.32`) imply you're addressing a named resource that exists in your system — like `/users/123`. Weather at a coordinate is not a stored resource.
- Request body is for `POST`/`PUT` where data is being created or mutated. We're only reading.
- Query params are composable, optional by design, and exactly suited for parameterizing a fetch.

**HTTP method: GET** — no mutation, pure read.

```
GET /weather?lat=27.7172&lng=85.3240
```

---

### 2. `internal/handler/handler.go` — Wire the Weather Client to HTTP

```go
package handler

import (
    "encoding/json"
    "net/http"
    "strconv"

    "github.com/Saugat-Tamang17/weather-wrapper/internal/weather"
)

type WeatherHandler struct {
    client *weather.Client
}

func New(client *weather.Client) *WeatherHandler {
    return &WeatherHandler{client: client}
}

func (h *WeatherHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    latStr := r.URL.Query().Get("lat")
    lngStr := r.URL.Query().Get("lng")

    if latStr == "" || lngStr == "" {
        http.Error(w, "lat and lng are required", http.StatusBadRequest)
        return
    }

    lat, err := strconv.ParseFloat(latStr, 64)
    if err != nil {
        http.Error(w, "invalid lat", http.StatusBadRequest)
        return
    }

    lng, err := strconv.ParseFloat(lngStr, 64)
    if err != nil {
        http.Error(w, "invalid lng", http.StatusBadRequest)
        return
    }

    coords := weather.Coordinates{Latitude: lat, Longitude: lng}

    result, err := h.client.GetWeather(coords)
    if err != nil {
        log.Printf("weather fetch failed: %v", err)
        http.Error(w, "failed to fetch weather", http.StatusBadGateway)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}
```

---

### 3. Why 502 and Not 500

- **500 Internal Server Error** — your service broke. The fault is yours.
- **502 Bad Gateway** — your service is fine, but an upstream dependency didn't respond correctly.

Using 502 when Open-Meteo fails tells clients and monitoring tools exactly where the fault is without any ambiguity. That distinction matters when you're debugging at 2am.

---

### 4. Information Disclosure — Never Leak Internal Errors to Clients

Returning `err.Error()` to the client exposes your stack, dependencies, and failure modes. For example:

```
Get "https://api.open-meteo.com/...": context deadline exceeded
```

That tells an attacker exactly what you're calling and how it fails. The rule: **log the full error server-side, send a generic message to the client.**

```go
log.Printf("weather fetch failed: %v", err)
http.Error(w, "failed to fetch weather", http.StatusBadGateway)
```

---

### 5. Silent Decoding Bug — Zero Values Are Not Errors in Go

The service was returning real JSON with all zero values:

```json
{"temperature_2m": 0, "windspeed_10m": 0, "weathercode": 0}
```

**Root cause:** Open-Meteo updated their API. The response fields changed:
- `current_weather` → `current`
- `temperature` → `temperature_2m`
- `windspeed` → `windspeed_10m`

Go's `json.Decoder` does not error on unknown or missing fields — it silently leaves them as zero values. This is one of the most common production bugs in Go services consuming external APIs.

**Fix in `models.go`:**
```go
type CurrentWeather struct {
    Temperature float64 `json:"temperature_2m"`
    Windspeed   float64 `json:"windspeed_10m"`
    WeatherCode int     `json:"weathercode"`
    IsDay       int     `json:"is_day"`
    Humidity    int     `json:"relative_humidity_2m"`
}

type WeatherResponse struct {
    Latitude  float64        `json:"latitude"`
    Longitude float64        `json:"longitude"`
    Timezone  string         `json:"timezone"`
    Current   CurrentWeather `json:"current"`
}
```

**Fix in `client.go` query params:**
```go
params.Set("current", "temperature_2m,windspeed_10m,weathercode,is_day,relative_humidity_2m")
```

---

### 6. In-Memory TTL Cache

Weather data doesn't change every second. Hitting Open-Meteo on every request is wasteful, slow, and leaves you at their mercy if they throttle you. A TTL cache serves repeat requests from memory and protects the upstream.

**Cache entry struct:**
```go
type cacheEntry struct {
    response  *WeatherResponse
    fetchedAt time.Time
}
```

**Cache fields added to `Client`:**
```go
type Client struct {
    httpClient *http.Client
    baseURL    string
    cacheTTL   time.Duration
    cache      map[string]cacheEntry
    mu         sync.RWMutex
}
```

**Cache key:** `"lat,lng"` string — unique per coordinate pair.

---

### 7. Concurrency Safety — `sync.RWMutex`

Go's HTTP server handles every request in its own goroutine automatically. Go's `map` is not safe for concurrent reads and writes — simultaneous access causes a data race, which corrupts data or panics the process.

**Fix: `sync.RWMutex`**

- `RLock` / `RUnlock` — multiple goroutines can read simultaneously
- `Lock` / `Unlock` — exclusive, nobody reads or writes while you're writing

```go
// Cache read
c.mu.RLock()
entry, ok := c.cache[key]
c.mu.RUnlock()

if ok && time.Since(entry.fetchedAt) < c.cacheTTL {
    return entry.response, nil
}

// ... fetch from upstream ...

// Cache write
c.mu.Lock()
c.cache[key] = cacheEntry{response: &result, fetchedAt: time.Now()}
c.mu.Unlock()
```

**To verify no data races:**
```bash
go run -race main.go
```

---

## Final `client.go`

```go
func (c *Client) GetWeather(coords Coordinates) (*WeatherResponse, error) {
    key := fmt.Sprintf("%f,%f", coords.Latitude, coords.Longitude)

    c.mu.RLock()
    entry, ok := c.cache[key]
    c.mu.RUnlock()

    if ok && time.Since(entry.fetchedAt) < c.cacheTTL {
        return entry.response, nil
    }

    params := url.Values{}
    params.Set("latitude", fmt.Sprintf("%f", coords.Latitude))
    params.Set("longitude", fmt.Sprintf("%f", coords.Longitude))
    params.Set("current", "temperature_2m,windspeed_10m,weathercode,is_day,relative_humidity_2m")

    fullURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())

    resp, err := c.httpClient.Get(fullURL)
    if err != nil {
        return nil, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("upstream returned %d", resp.StatusCode)
    }

    var result WeatherResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decode failed: %w", err)
    }

    c.mu.Lock()
    c.cache[key] = cacheEntry{response: &result, fetchedAt: time.Now()}
    c.mu.Unlock()

    return &result, nil
}
```

---

## What We Did Not Do Yet (Next Session)

- Verify cache is working (add temp `log.Println("cache hit")`, hit same coords twice)
- Run with `-race` flag to confirm no data races
- Docker

---

## Summary Prompt (Paste This at the Start of Next Session)

```
SYSTEM PROMPT (Senior Backend Engineer Mode)
You are a senior backend software engineer with 10+ years of industry experience working in production systems. Your role is to teach me backend development the way it is taught in real software teams, not like a tutorial blog. Be direct, honest, and practical. Correct me clearly when I'm wrong. Focus on how things actually work in production. No diagrams, no excessive formatting, no filler. Everything in Golang.

SESSION CONTEXT:
We are building a weather API wrapper service in Go that calls Open-Meteo (free, no API key).

Completed so far:
- go mod init, project folder structure (internal/, config/, handler/, weather/)
- main.go: http.NewServeMux, /health endpoint, /weather route, ListenAndServe with error check
- config/config.go: loads PORT and CACHE_TTL_SECONDS from env with defaults, OpenMeteoURL hardcoded in config
- internal/weather/models.go: Coordinates, CurrentWeather, WeatherResponse structs with corrected JSON tags (temperature_2m, windspeed_10m, current)
- internal/weather/client.go: Client struct with custom http.Client (10s timeout), TTL cache with sync.RWMutex, GetWeather method with url.Values, defer resp.Body.Close(), error wrapping with %w
- internal/handler/handler.go: parses lat/lng query params, validates and parses to float64, calls weather client, returns JSON, uses 502 for upstream failures

Next steps:
1. Verify cache is working — add temp log.Println("cache hit"), hit same coords twice, confirm second request does not call upstream
2. Run with -race flag to confirm no data races under concurrent load
3. Docker — containerize the service

Resume from there.
```
