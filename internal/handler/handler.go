package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/Saugat-Tamang17/weather-wrapper/internal/weather"
)

type WeatherService interface {
	GetWeather(coords weather.Coordinates) (*weather.WeatherResponse, error)
}

type WeatherHandler struct {
	client WeatherService
}

func New(client WeatherService) *WeatherHandler {
	return &WeatherHandler{client: client}
}

func (h *WeatherHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	latsstr := r.URL.Query().Get("lat")
	longstr := r.URL.Query().Get("lng")

	if latsstr == "" || longstr == "" {
		http.Error(w, "Latitude and Longitude are both required", http.StatusBadRequest)
		return
	}

	long, err := strconv.ParseFloat(longstr, 64)
	if err != nil {
		http.Error(w, "Invalid Longitude value :", http.StatusBadRequest)
		return
	}

	lat, err := strconv.ParseFloat(latsstr, 64)
	if err != nil {
		http.Error(w, "Invalid Latitude value:", http.StatusBadRequest)
		return
	}

	if lat < -90 || lat > 90 {
		http.Error(w, "latitude must be between -90 and 90", http.StatusBadRequest)
		return
	}

	if long < -180 || long > 180 {
		http.Error(w, "Longtitude value must be between -180 and 180", http.StatusBadRequest)
		return
	}
	coords := weather.Coordinates{
		Latitude:  lat,
		Longitude: long,
	}

	result, err := h.client.GetWeather(coords)
	if err != nil {
		http.Error(w, "failed to fetch the weather", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("failed to encode response: %v", err)
	}

}
