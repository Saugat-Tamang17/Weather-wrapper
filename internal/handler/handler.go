package handler

import (
	"net/http"
"encoding/json"
"strconv"
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
		http.Error(w, "Latitude and Longitude are nboth required",http.StatusBadRequest)
		return
	}

	long,err:=strconv.ParseFloat(longstr,64)
	if err !=nil{
		http.Error(w,"Invalid Longitude value :",http.StatusBadRequest)
		return
	}

	lat, err :=strconv.ParseFloat(latsstr,64)
	if err !=nil{
		http.Error(w,"Invalid Latitude value:",http.StatusBadRequest)
	}
	
