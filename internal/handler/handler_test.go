package handler

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/Saugat-Tamang17/weather-wrapper/internal/weather"
)

type fakeClient struct {
	response *weather.WeatherResponse
	err      error
}

func (f *fakeClient) GetWeather(coords weather.Coordinates) (*weather.WeatherResponse, error) {
	return f.response, f.err
}

func TestWeatherHandler(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		fakeResp   *weather.WeatherResponse
		fakeErr    error
		wantStatus int
	}{
		{
			name:       "missing params",
			url:        "/weather",
			wantStatus: 400,
		},
		{
			name:       "Invalid Lat",
			url:        "/weather?lat=abc&lng=10",
			wantStatus: 400,
		},
		{
			name:       "Invalid Long",
			url:        "/weather?lat=57&lng=abc",
			wantStatus: 400,
		},
		{
			name:       "lat value too low",
			url:        "/weather?lat=-999&lng=10",
			wantStatus: 400,
		},
		{
			name:       "lat value too high",
			url:        "/weather?lat=999&lng=10",
			wantStatus: 400,
		},
		{
			name:       "long value too low",
			url:        "/weather?lat=10&lng=-999",
			wantStatus: 400,
		},
		{
			name:       "long value too high",
			url:        "/weather?lat=10&lng=999",
			wantStatus: 400,
		},
		{
			name:       "service Failure",
			url:        "/weather?lat=10&lng=10",
			fakeErr:    errors.New("api down"),
			wantStatus: 502,
		},
		{
			name:       "happy path",
			url:        "/weather?lat=10&lng=20",
			fakeResp:   &weather.WeatherResponse{},
			wantStatus: 200,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {

			// 1. create fake client
			fake := &fakeClient{
				response: tt.fakeResp,
				err:      tt.fakeErr,
			}

			// 2. create handler
			h := New(fake)

			// 3. create request
			req := httptest.NewRequest("GET", tt.url, nil)

			// 4. create recorder
			rec := httptest.NewRecorder()

			// 5. call handler
			h.ServeHTTP(rec, req)

			// 6. assert status
			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
