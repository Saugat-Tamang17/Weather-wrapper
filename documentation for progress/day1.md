# Backend Development with Go — Session 01
**Project:** Weather API Wrapper Service  
**Stack:** Golang, Open-Meteo API  
**Date:** 2026-05-02

---

## What We Built

A structured Go HTTP service that wraps the Open-Meteo weather API. By end of session the project had a running HTTP server, environment-based config, a typed HTTP client with timeout, proper error wrapping, and resource cleanup.

---

## Project Structure

```
weather-wrapper/
├── main.go
├── go.mod
├── internal/
│   ├── weather/
│   │   ├── client.go
│   │   └── models.go
│   └── handler/
│       └── handler.go
└── config/
    └── config.go
```

**Why this structure:**

- `internal/` is enforced by the Go compiler — nothing outside this module can import it. It signals "this is not a public API."
- `handler/` owns HTTP concerns only.
- `weather/` owns external API communication only.
- `config/` owns environment and settings.
- `main.go` just wires everything together and starts the server. No business logic lives here.

---

## Go Modules

```bash
go mod init github.com/yourname/weather-wrapper
```

- `go.mod` tracks your module name and Go version.
- `go.sum` locks exact dependency checksums — that is your reproducible build guarantee.
- The module path (`github.com/yourname/...`) is a convention matching the actual repo URL so imports are unambiguous across services. It does not have to exist on GitHub to work locally.

---

## Concepts Covered

### 1. `http.ListenAndServe` and why we check its error

`ListenAndServe` blocks and runs continuously as long as the server is healthy. It only returns when something fatal happens — port conflict, OS-level error, etc. We use `log.Fatalf` so the process dies loudly with a non-zero exit code. In production that exit code tells your process manager (systemd, Kubernetes) that something went wrong and it needs to restart or alert. Silent failures are how outages go unnoticed for hours.

```go
if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
    log.Fatalf("server failed: %v", err)
}
```

### 2. Config separation (12-factor principle)

Never hardcode values like ports or URLs inside business logic. Read them from the environment. The reason: same binary, different behavior per environment. You do not rebuild to change an endpoint — you change an env var and restart. That is what makes deployments safe and fast.

```go
port := os.Getenv("PORT")
if port == "" {
    port = "8080"
}
```

Config also lives in one place, which makes it testable — in tests you point `OpenMeteoURL` at a mock server instead of hitting the real API.

**Windows PowerShell syntax for env vars:**
```powershell
$env:PORT=9090; go run main.go
```

**Linux/Mac bash syntax:**
```bash
PORT=9090 go run main.go
```

### 3. Custom `http.Client` vs `http.Get()`

`http.Get()` uses Go's default HTTP client which has no timeout. If the upstream API hangs, your goroutine hangs with it indefinitely. At scale you accumulate hanging goroutines until the service dies.

Always define your own client with an explicit timeout:

```go
httpClient: &http.Client{
    Timeout: 10 * time.Second,
}
```

### 4. `defer resp.Body.Close()`

Releases the network connection and memory buffer associated with the response once you are done reading it. If you forget it in a service handling hundreds of requests per minute, goroutines pile up holding open buffers, producing a memory leak that only shows up under load. Classic production gotcha.

```go
resp, err := c.httpClient.Get(fullURL)
if err != nil {
    return nil, fmt.Errorf("request failed: %w", err)
}
defer resp.Body.Close()
```

### 5. Error wrapping with `%w`

Using `%w` instead of `%v` in `fmt.Errorf` wraps the original error, preserving the full error chain. This means callers can use `errors.Is()` and `errors.As()` to inspect the underlying cause. Always wrap errors as they travel up the call stack so you have full context when they surface in logs.

```go
return nil, fmt.Errorf("request failed: %w", err)
```

### 6. Struct tags and JSON mapping

```go
type CurrentWeather struct {
    Temperature float64 `json:"temperature"`
    Windspeed   float64 `json:"windspeed"`
    WeatherCode int     `json:"weathercode"`
}
```

Struct tags tell the JSON decoder how to map API response fields to Go struct fields. We only map fields we actually use — mapping the entire API response creates maintenance burden with no benefit.

The nested struct structure mirrors the JSON shape intentionally. `WeatherResponse.CurrentWeather.Temperature` tells a reader immediately how the upstream data is organized.

### 7. `url.Values` for query parameters

Never build query strings by hand with string concatenation. Use `url.Values` — it handles encoding, escaping, and edge cases correctly.

```go
params := url.Values{}
params.Set("latitude", fmt.Sprintf("%f", coords.Latitude))
params.Set("longitude", fmt.Sprintf("%f", coords.Longitude))
params.Set("current_weather", "true")

fullURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())
```

---

## Code Written This Session

### `config/config.go`
Loads PORT, CACHE_TTL_SECONDS from environment with safe defaults.

### `internal/weather/models.go`
Typed structs for coordinates and Open-Meteo API response.

### `internal/weather/client.go`
HTTP client with timeout, query param building, response decoding, and proper error wrapping.

### `main.go`
Server bootstrap, config loading, ServeMux setup, `/health` endpoint.

---

## What We Did Not Do Yet (Next Session)

- `internal/handler/handler.go` — wire the weather client to a `/weather` HTTP endpoint
- In-memory cache with TTL so we are not hammering Open-Meteo on every request
- Docker (after the service is functionally complete)

---

## Open Question for Next Session

What should the `/weather` endpoint accept — query params, path params, or a request body? What HTTP method makes sense and why? Have a reason ready, not just a guess.

---

## Why Docker Was Deferred

Docker is a packaging and deployment tool. Learning it before you understand what is inside the box is backwards. The correct order: write a working service, understand what it needs to run (runtime, env vars, ports), then containerize it. Go binaries are trivially easy to containerize — it will take about 15 minutes once the service is solid.

---

## Summary Prompt (Paste This at the Start of Next Session)

```
SYSTEM PROMPT (Senior Backend Engineer Mode)
You are a senior backend software engineer with 10+ years of industry experience working in production systems. Your role is to teach me backend development the way it is taught in real software teams, not like a tutorial blog. Be direct, honest, and practical. Correct me clearly when I'm wrong. Focus on how things actually work in production. No diagrams, no excessive formatting, no filler. Everything in Golang.

SESSION CONTEXT:
We are building a weather API wrapper service in Go that calls Open-Meteo (free, no API key).

Completed so far:
- go mod init, project folder structure (internal/, config/, handler/, weather/)
- main.go: http.NewServeMux, /health endpoint, ListenAndServe with error check
- config/config.go: loads PORT and CACHE_TTL_SECONDS from env with defaults, OpenMeteoURL hardcoded in config
- internal/weather/models.go: Coordinates, CurrentWeather, WeatherResponse structs with JSON tags
- internal/weather/client.go: Client struct with custom http.Client (10s timeout), GetWeather method with url.Values, defer resp.Body.Close(), error wrapping with %w

Next steps:
1. Build internal/handler/handler.go — HTTP handler that parses lat/lng from request and calls the weather client
2. Add in-memory cache with TTL to the weather client
3. Docker

The student needs to answer this before we proceed: what should the /weather endpoint accept as input (query params, path params, or request body) and what HTTP method, and why?

Resume from there.
```
