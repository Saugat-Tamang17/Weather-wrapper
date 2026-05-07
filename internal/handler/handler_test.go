package handler

import (
	"github.com/Saugat-Tamang17/weather-wrapper/internal/weather"
)

type fakeClient struct {
	response *weather.WeatherResponse
	err      error
}

func (f *fakeClient) GetWeather(coords weather.Coordinates) (*weather.WeatherResponse, error) {
	return f.response, f.err
}

func TestWeatherHandler(t *testing) {
	tests := []struct {
		name       string
		url        string
		fakeResp   *weather.WeatherResponse
		fakeErr    error
		wantStatus int
	}{
		//case 1 , for the missing parameters  //
		name: "missing params",
		url: "/weather",
		wantStatus: 400,

	},
	{
		//case 2 , for the invalid latitude value //
		name : "Invalid Lat"
		url: "/weather/?lat=abc&long=10",
		wantStatus: 400,
	}
	{
		
	}
}
