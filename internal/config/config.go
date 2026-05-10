package config

import (
	"log"
	"os"
	"strconv"
)

type config struct {
	Port           string
	APIURL         string
	WeatherFields  string
	CacheTime      int
	RateLimitRate  float64
	RateLimitBurst int
}

func Load() *config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "5050"
	}

	ttl := 300 //ttl is the refresh time //
	if v := os.Getenv("Cache_time"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			log.Printf("Invalid Cache_TTL_second value . Resorting to the default Value\n the error was :%v", err)
		} else {
			ttl = parsed
		}
	}

	return &config{
		Port:          port,
		APIURL:        "https://api.open-meteo.com/v1/forecast",
		WeatherFields: "temperature_2m,windspeed_10m,relative_humidity_2m,weathercode,is_day",
		CacheTime:     ttl,
	}
}
