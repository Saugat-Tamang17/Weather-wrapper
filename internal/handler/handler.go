package handler

import (
	"net/http"

	"github.com/Saugat-Tamang17/weather-wrapper/internal/weather"
)

type WeatherHandler struct {
	client *weather.Client
}

func New(client *weather.Client) *WeatherHandler {
	return &WeatherHandler{client: client}
}

func (h *WeatherHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	latsstr := r.URL.Query().Get("lat")
	longstr := r.URL.Query().Get("lng")

	if latsstr == "" || longstr == "" {
		http.Error(w, "Latitude and Longitude  ")
	}
}
