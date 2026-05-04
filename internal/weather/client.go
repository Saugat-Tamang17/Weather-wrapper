package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type cacheEntry struct {
	response  *WeatherResponse
	fetchedAt time.Time
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	cacheTTL   time.Duration
	cache      map[string]cacheEntry
	mu         sync.RWMutex
}

func NewClient(baseURL string, cacheTTLSeconds int) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cacheTTL:   time.Duration(cacheTTLSeconds) * time.Second,
		cache:      make(map[string]cacheEntry),
	}
}

func (c *Client) GetWeather(coords Coordinates) (*WeatherResponse, error) {
	key := fmt.Sprintf("%f,%f", coords.Latitude, coords.Longitude)

	c.mu.RLock()
	if entry, ok := c.cache[key]; ok {
		if time.Since(entry.fetchedAt) < c.cacheTTL {
			c.mu.RUnlock()
			return entry.response, nil
		}
	}
	c.mu.RUnlock()

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
	c.cache[key] = cacheEntry{
		response:  &result,
		fetchedAt: time.Now(),
	}
	c.mu.Unlock()

	return &result, nil
}
