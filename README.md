# Weather Wrapper

A production-grade HTTP API service written in Go that wraps the [Open-Meteo](https://open-meteo.com/) free weather API. Built from scratch over 7 sessions as a real backend engineering exercise — no tutorials, no shortcuts.

**→ [github.com/Saugat-Tamang17/Weather-wrapper](https://github.com/Saugat-Tamang17/Weather-wrapper)**

---

## What It Does

Send it a latitude and longitude, get back current weather data. That's the surface. Under the hood it's a fully production-hardened service: every request is rate-limited, cached, assigned a unique ID, logged, and protected against panics — all before it touches your handler logic.

---

## Stack

- **Language** — Go 1.24
- **Weather data** — Open-Meteo (free, no API key required)
- **Containerisation** — Docker multi-stage build
- **Dependencies** — `golang.org/x/time/rate` for the token bucket. Everything else is stdlib.

---

## API

```
GET /weather?lat={latitude}&lng={longitude}
GET /health
```

**Example request**
```
GET /weather?lat=27.7172&lng=85.3240
```

**Example response**
```json
{
  "latitude": 27.7172,
  "longitude": 85.324,
  "current_weather": {
    "temperature": 28.4,
    "windspeed": 12.1,
    "weathercode": 3
  }
}
```

**Response codes**

| Code | Meaning |
|------|---------|
| `200` | Success |
| `400` | Missing or invalid coordinates |
| `429` | Rate limit exceeded — check `Retry-After` header |
| `502` | Upstream Open-Meteo failure |
| `500` | Internal server error (panic recovered) |

---

## Highlights

### Per-IP Token Bucket Rate Limiter
Each client IP gets its own token bucket — 5 tokens/sec, burst of 10. A fresh client can make 10 rapid requests before being throttled. Sustained abuse gets 429s with a `Retry-After` header. Stale IP entries are evicted every 5 minutes to prevent unbounded memory growth. Rate and burst are configurable via environment variables.

### Concurrent-Safe TTL Cache
Weather responses are cached in-memory with a `sync.RWMutex` — multiple readers can hit the cache simultaneously, writes are exclusive. Identical coordinate requests skip Open-Meteo entirely until TTL expires. TTL is configurable via `CACHE_TTL_SECONDS`.

### Middleware Chain
Every request passes through a fixed chain before reaching the handler:

```
Recovery → Logger → RequestID → RateLimit → mux/handler
```

- **Recovery** — outermost, always. Catches any panic downstream, logs the stack trace, returns 500. The server never crashes.
- **Logger** — logs every request including 429s from the rate limiter.
- **RequestID** — generates a unique ID per request, injects it into context. Every log line carries it.
- **RateLimit** — per-IP token bucket, rejects with 429 + `Retry-After` when the bucket is empty.

### Graceful Shutdown
Listens for `SIGINT` / `SIGTERM`. On signal, stops accepting new connections and waits up to 15 seconds for in-flight requests to complete before exiting cleanly. Works correctly with `docker stop`.

### Docker Multi-Stage Build
Builder stage compiles a fully static binary (`CGO_ENABLED=0`, `GOOS=linux`). Runner stage is a minimal `alpine:3.21` image with only CA certificates added. No Go toolchain ships in the final image.

---

## Project Structure

```
.
├── main.go
├── Dockerfile
├── config/
│   └── config.go           # env-based config with defaults
├── internal/
│   ├── handler/
│   │   ├── handler.go      # request parsing, validation, response
│   │   └── handler_test.go # 9 table-driven test cases
│   ├── middleware/
│   │   ├── middleware.go   # RequestID, Logger
│   │   ├── recovery.go     # panic recovery
│   │   └── ratelimit.go    # per-IP token bucket + IP eviction
│   └── weather/
│       ├── client.go       # HTTP client, TTL cache
│       ├── models.go       # request/response structs
│       └── client_test.go  # happy path, upstream 503, cache hit
```

---

## Running It

**With Docker**
```bash
docker build -t weather-wrapper .
docker run -p 8080:8080 weather-wrapper
```

**With Go**
```bash
go run main.go
```

**Configuration** — all optional, defaults shown
```bash
PORT=8080
CACHE_TTL_SECONDS=60
RATE_LIMIT_RATE=5       # tokens per second
RATE_LIMIT_BURST=10     # max burst size
```

**Run tests**
```bash
go test ./...
```

---

## Status

| Area | Status |
|------|--------|
| Project structure | ✅ Done |
| HTTP handler + validation | ✅ Done |
| TTL cache with RWMutex | ✅ Done |
| Graceful shutdown | ✅ Done |
| Docker multi-stage build | ✅ Done |
| Middleware (Logger, RequestID) | ✅ Done |
| Panic recovery middleware | ✅ Done |
| Input validation | ✅ Done |
| Handler unit tests | ✅ Done |
| Weather client unit tests | ✅ Done |
| Rate limiting | ✅ Done |
| Retry-After header on 429 | ✅ Done |
| Rate/burst configurable via env | ✅ Done |
| IP eviction | ✅ Done |
| Rate limiter tests | 🔜 Pending |

---

*Built in Go. No frameworks. No magic.*
