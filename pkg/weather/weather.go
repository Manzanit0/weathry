package weather

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type Client interface {
	GetCurrentWeather(lat, lon float64) (*Forecast, error)
	GetUpcomingWeather(lat, lon float64) ([]*Forecast, error)
}

type Coordinates struct {
	Latitude  float64
	Longitude float64
}

type Forecast struct {
	Coordinates        Coordinates
	Location           string
	Condition          string
	Description        string
	MinimumTemperature float64
	MaximumTemperature float64
	Humidity           int
	WindSpeed          float64
	DateTimeTS         int
}

func (f *Forecast) IsRainy() bool {
	return f.Condition == "rain" || f.Condition == "storm"
}

func (f *Forecast) FormattedDateTime() string {
	return time.Unix(int64(f.DateTimeTS), 0).Format(time.RFC1123)
}

func NewOpenWeatherMapClient(h *http.Client, apiKey string) Client {
	return &owm{h: h, apiKey: apiKey}
}

type owm struct {
	h      *http.Client
	apiKey string
}

func (c *owm) GetCurrentWeather(lat, lon float64) (*Forecast, error) {
	forecasts, err := c.GetUpcomingWeather(lat, lon)
	if err != nil {
		return nil, err
	}

	return forecasts[0], nil
}

func (c *owm) GetUpcomingWeather(lat, lon float64) ([]*Forecast, error) {
	endpoint := fmt.Sprintf("/data/2.5/forecast/daily/?lat=%f&lon=%f&units=metric&lang=es", lat, lon)
	url := fmt.Sprintf("http://api.openweathermap.org%s&appid=%s", endpoint, c.apiKey)

	res, err := c.h.Get(url)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var d DailyWeatherResponse
	err = json.Unmarshal(data, &d)
	if err != nil {
		return nil, err
	}

	var forecasts []*Forecast
	for _, v := range d.DaysList {
		forecasts = append(forecasts, &Forecast{
			Coordinates:        Coordinates{lat, lon},
			Location:           fmt.Sprintf("%s (%s)", d.City.Name, d.City.Country),
			Description:        v.Weather[0].Description,
			MinimumTemperature: v.Temperature.Min,
			MaximumTemperature: v.Temperature.Max,
			Humidity:           v.Humidity,
			WindSpeed:          v.Speed,
			DateTimeTS:         v.DateTimeTS,
			Condition:          d.Condition(),
		})
	}

	return forecasts, nil
}

type DailyWeatherResponse struct {
	City struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Coordinates struct {
			Lon float64 `json:"lon"`
			Lat float64 `json:"lat"`
		} `json:"coord"`
		Country    string `json:"country"`
		Population int    `json:"population"`
		Timezone   int    `json:"timezone"`
	} `json:"city"`
	Cod       string  `json:"cod"`
	Message   float64 `json:"message"`
	DaysCount int     `json:"cnt"`
	DaysList  []struct {
		DateTimeTS  int `json:"dt"`
		Sunrise     int `json:"sunrise"`
		Sunset      int `json:"sunset"`
		Temperature struct {
			Day   float64 `json:"day"`
			Min   float64 `json:"min"`
			Max   float64 `json:"max"`
			Night float64 `json:"night"`
			Eve   float64 `json:"eve"`
			Morn  float64 `json:"morn"`
		} `json:"temp"`
		FeelsLikeTemperature struct {
			Day   float64 `json:"day"`
			Night float64 `json:"night"`
			Eve   float64 `json:"eve"`
			Morn  float64 `json:"morn"`
		} `json:"feels_like"`
		Pressure float64 `json:"pressure"`
		Humidity int     `json:"humidity"`
		Weather  []struct {
			ID          int    `json:"id"`
			Main        string `json:"main"`
			Description string `json:"description"`
			Icon        string `json:"icon"`
		} `json:"weather"`
		Speed  float64 `json:"speed"`
		Deg    int     `json:"deg"`
		Gust   float64 `json:"gust"`
		Clouds int     `json:"clouds"`
		Pop    float64 `json:"pop"`
	} `json:"list"`
}

func (r DailyWeatherResponse) Condition() string {
	// TODO: make this configurable
	code := r.DaysList[0].Weather[0].ID

	if code >= 200 && code <= 299 {
		return "thunderstorm"
	} else if code >= 300 && code <= 399 {
		return "drizzle"
	} else if code >= 500 && code <= 599 {
		return "rain"
	} else if code >= 600 && code <= 699 {
		return "snow"
	} else if code >= 700 && code <= 799 {
		return "atmosphere"
	} else if code == 800 {
		return "clear"
	} else if code >= 801 && code <= 899 {
		return "clouds"
	} else {
		return ""
	}
}
