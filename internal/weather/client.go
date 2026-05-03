package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
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
