package config

import (
	"log"
	"os"
	"strconv"
)

type config struct {
	Port      string
	APIURL    string
	CacheTime int
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
		Port:      port,
		APIURL:    "https://api.open-meteo.com/v1/forecast",
		CacheTime: ttl,
	}
}
