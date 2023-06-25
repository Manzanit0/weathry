package action

import (
	"fmt"

	"github.com/manzanit0/weathry/cmd/bot/msg"
	"github.com/manzanit0/weathry/pkg/location"
	"github.com/manzanit0/weathry/pkg/weather"
)

func GetDailyWeather(locClient location.Client, weatherClient weather.Client, query string) (string, error) {
	location, err := locClient.FindLocation(query)
	if err != nil {
		return "", fmt.Errorf("find location: %w", err)
	}

	forecasts, err := weatherClient.GetUpcomingWeather(location.Latitude, location.Longitude)
	if err != nil {
		return "", fmt.Errorf("get weather: %w", err)
	}

	return msg.NewForecastTableMessage(location, forecasts, msg.WithTemperatureDiff()), nil
}

func GetDailyWeatherByCoordinates(locClient location.Client, weatherClient weather.Client, latitude, longitude float64) (string, error) {
	location, err := locClient.ReverseFindLocation(latitude, longitude)
	if err != nil {
		return "", fmt.Errorf("find location: %w", err)
	}

	forecasts, err := weatherClient.GetUpcomingWeather(location.Latitude, location.Longitude)
	if err != nil {
		return "", fmt.Errorf("get weather: %w", err)
	}

	return msg.NewForecastTableMessage(location, forecasts, msg.WithTemperatureDiff()), nil
}

func GetHourlyWeatherByLocationName(locClient location.Client, weatherClient weather.Client, locationName string) (string, error) {
	location, err := locClient.FindLocation(locationName)
	if err != nil {
		return "", fmt.Errorf("find location: %w", err)
	}

	return getHourlyWeather(weatherClient, location)
}

func GetHourlyWeatherByCoordinates(locClient location.Client, weatherClient weather.Client, latitude, longitude float64) (string, error) {
	location, err := locClient.ReverseFindLocation(latitude, longitude)
	if err != nil {
		return "", fmt.Errorf("find location: %w", err)
	}

	return getHourlyWeather(weatherClient, location)
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
