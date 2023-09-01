package services

import (
	"fmt"

	"github.com/manzanit0/weathry/cmd/bot/msg"
	"github.com/manzanit0/weathry/pkg/location"
	"github.com/manzanit0/weathry/pkg/weather"
)

type WeatherService struct {
	locationClient location.Client
	weatherClient  weather.Client
}

func NewWeatherService(l location.Client, w weather.Client) *WeatherService {
	return &WeatherService{locationClient: l, weatherClient: w}
}

func (a *WeatherService) GetDailyWeatherByLocationName(locationName string) (string, error) {
	location, err := a.locationClient.FindLocation(locationName)
	if err != nil {
		return "", fmt.Errorf("find location: %w", err)
	}

	forecasts, err := a.weatherClient.GetUpcomingWeather(location.Latitude, location.Longitude)
	if err != nil {
		return "", fmt.Errorf("get weather: %w", err)
	}

	return msg.NewForecastTableMessage(location, forecasts, msg.WithTemperatureDiff()), nil
}

func (a *WeatherService) GetDailyWeatherByCoordinates(latitude, longitude float64) (string, error) {
	location, err := a.locationClient.ReverseFindLocation(latitude, longitude)
	if err != nil {
		return "", fmt.Errorf("find location: %w", err)
	}

	forecasts, err := a.weatherClient.GetUpcomingWeather(location.Latitude, location.Longitude)
	if err != nil {
		return "", fmt.Errorf("get weather: %w", err)
	}

	return msg.NewForecastTableMessage(location, forecasts, msg.WithTemperatureDiff()), nil
}

func (a *WeatherService) GetHourlyWeatherByLocationName(locationName string) (string, error) {
	location, err := a.locationClient.FindLocation(locationName)
	if err != nil {
		return "", fmt.Errorf("find location: %w", err)
	}

	return getHourlyWeather(a.weatherClient, location)
}

func (a *WeatherService) GetHourlyWeatherByCoordinates(latitude, longitude float64) (string, error) {
	location, err := a.locationClient.ReverseFindLocation(latitude, longitude)
	if err != nil {
		return "", fmt.Errorf("find location: %w", err)
	}

	return getHourlyWeather(a.weatherClient, location)
}

func getHourlyWeather(weatherClient weather.Client, location *location.Location) (string, error) {
	forecasts, err := weatherClient.GetHourlyForecast(location.Latitude, location.Longitude)
	if err != nil {
		return "", fmt.Errorf("get weather: %w", err)
	}

	// Just 9 forecasts for the hourly, to cover 24h.
	if len(forecasts) > 9 {
		filtered := make([]*weather.Forecast, 9)
		for i := 0; i < 9; i++ {
			filtered[i] = forecasts[i]
		}

		return msg.NewForecastTableMessage(location, filtered, msg.WithTime()), nil
	}

	// We don't need the temperature diff because within the hour there's not much difference.
	return msg.NewForecastTableMessage(location, forecasts, msg.WithTime()), nil
}
