package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/Saugat-Tamang17/weather-wrapper/internal/config"
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

func NewClient(cfg *config.Config) *Client {
	return &Client{
		baseURL:    cfg.OpenMeteoURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cacheTTL:   time.Duration(cfg.CacheTTLSeconds) * time.Second,
		cache:      make(map[string]cacheEntry),
	}
}

func (c *Client) GetWeather(coords Coordinates) (*WeatherResponse, error) {
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

	return &result, nil
}
