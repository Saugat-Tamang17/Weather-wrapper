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
		//case 3 , for the invalid longitude value //
		name: "Invalid Long",
		url : "/weather/?lat=57&long=abc",
		wantStatus:400,
	}
	{
		//case 4 ,for the latitude value being too low //
		name:"lat value too low",
		url :"/weather/?lat=-999&long=10"
		wantStatus:400,
	}
	{
		//case 5, for the latitude value being too high //
		name:"lat value too high"
		url :"/weather/?lat=999&long=10"
		wantStatus: 400,
	}
	{
	//case 6, for the longitude value being too low //
		name:"long value too high"
		url :"/weather/?lat=10&long=-999"
		wantStatus: 400,
	}
	{
//case 7, for the longitude value being too high //
		name:"long value too high"
		url :"/weather/?lat=10&long=999"
		wantStatus: 400,
	}

	{
		//case 8, for the service failure//
		name:"service Failure"
		url :"/weather/?lat=10&long=10",
		fakeErr:errors.New("api down")
		wantStatus: 502,
	}
	{
		//case 9, for the happy path //
		name:       "happy path",
		url:        "/weather?lat=10&lng=20",
		fakeResp:   weather.Response{},
		wantStatus: 200,
	}
}
