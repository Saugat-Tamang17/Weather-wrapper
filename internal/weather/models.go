package weather

type CurrentWeather struct {
	Temperature float64 `json:"temperature_2m"`
	Windspeed   float64 `json:"windspeed_10m"`
	WeatherCode int     `json:"weathercode"`
	IsDay       int     `json:"is_day"`
	Humidity    float64 `json:"relative_humidity_2m"`
}

type WeatherResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`

	Current CurrentWeather `json:"current"`
}
