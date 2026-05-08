package weather

import (
	"net/http"
	"net/http/httptest"
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
