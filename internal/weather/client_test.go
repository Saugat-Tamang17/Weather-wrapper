package weather

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClient_GetWeather_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(
			`{
			"latitude":27.7,
			"longitude":85.3,
			"current":{
			"temperature_2m":22.5
			}
			}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, 60)

	resp, err := client.GetWeather(Coordinates{
		Latitude:  27.7,
		Longitude: 85.3,
	})
	if err != nil {
		t.Fatalf("Expected No Error , But Error Occured : %v", err)
	}

	if resp.Latitude != 27.7 {
		t.Errorf("latitude mismatch: got %v", resp.Latitude)
	}

	if resp.Longitude != 85.3 {
		t.Errorf("longitude mismatch: got %v", resp.Longitude)
	}
}
func TestClient_GetWeather_Upstream503(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL, 60)
	resp, err := client.GetWeather(Coordinates{
		Latitude:  27.7,
		Longitude: 85.3,
	})

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

func TestClient_GetWeather_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("this is not valid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, 60)

	resp, err := client.GetWeather(Coordinates{
		Latitude:  27.7,
		Longitude: 85.3,
	})

	if err == nil {
		t.Fatalf("expected error due to malformed JSON, got nil")
	}

	if resp != nil {
		t.Fatalf("expected nil response, got %+v", resp)
	}
}

func TestClient_GetWeather_CacheHit(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)

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

	coords := Coordinates{
		Latitude:  27.7,
		Longitude: 85.3,
	}

	_, err := client.GetWeather(coords)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// second call → should come from cache
	_, err = client.GetWeather(coords)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if callCount.Load() != 1 {
		t.Fatalf("expected 1 upstream call, got %d", callCount.Load())
	}

}
