package pings

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/manzanit0/weathry/cmd/bot/location"
	"github.com/manzanit0/weathry/pkg/geocode"
	"github.com/manzanit0/weathry/pkg/tgram"
	"github.com/manzanit0/weathry/pkg/weather"
)

type Pinger interface {
	MonitorWeather(context.Context) error
}

func NewBackgroundPinger(f weather.Client, g geocode.Client, t tgram.Client, l location.Repository) *backgroundPinger {
	return &backgroundPinger{forecaster: f, geocoder: g, telegram: t, locations: l}
}

type backgroundPinger struct {
	forecaster weather.Client
	geocoder   geocode.Client
	telegram   tgram.Client
	locations  location.Repository
}

func (p *backgroundPinger) MonitorWeather(ctx context.Context) error {
	return p.PingRainyForecasts()
}

func (p *backgroundPinger) PingRainyForecasts() error {
	homes, err := p.locations.ListHomes(context.Background())
	if err != nil {
		return fmt.Errorf("list homes: %w", err)
	}

	for _, home := range homes {
		logger := slog.
			Default().
			With("ctx.user_id", home.UserID).
			With("ctx.home", home.Name)

		var message string

		forecasts, err := p.forecaster.GetHourlyForecast(home.Latitude, home.Longitude)
		if err != nil {
			logger.Error("error requesting upcoming weather", "error", err.Error())
			continue
		}

		rainyForecast := FindNextRainyDay(forecasts)
		if rainyForecast != nil {
			if isToday(rainyForecast.DateTimeTS) {
				message = fmt.Sprintf("Heads up, it's going to be raining today at %s!", rainyForecast.FormattedTime())
			} else {
				message = fmt.Sprintf("Hey 👋! I'm expecting rain next %s at around %s.",
					rainyForecast.FormattedDate(),
					rainyForecast.FormattedTime())
			}
		}

		highTempForecast := FindNextHighTemperature(forecasts)
		if highTempForecast != nil {
			if len(message) > 0 {
				message += "\nAlso, on a separate note, "
			} else {
				message = "Hi! Just letting you know that "
			}

			if isToday(highTempForecast.DateTimeTS) {
				message += fmt.Sprintf("it's going to be pretty hot today with a max of %.2fºC! 🔥",
					highTempForecast.MaximumTemperature)
			} else {
				message += fmt.Sprintf("next %s temperatures are going to rise all the way to %.2fºC! 🔥",
					highTempForecast.FormattedDateTime(),
					highTempForecast.MaximumTemperature)
			}
		}

		lowTempForecast := FindNextLowTemperature(forecasts)
		if lowTempForecast != nil {
			if len(message) > 0 {
				message += "\nAlso, on a separate note, "
			} else {
				message = "Hi! Just letting you know that "
			}

			if isToday(lowTempForecast.DateTimeTS) {
				message += fmt.Sprintf("it's going to be pretty cold today with a min of %.2fºC! ❄️ ",
					lowTempForecast.MinimumTemperature)
			} else {
				message += fmt.Sprintf("next %s temperatures are going to decrease the way to %.2fºC! ❄️ ",
					lowTempForecast.FormattedDateTime(),
					lowTempForecast.MinimumTemperature)
			}
		}

		if message == "" {
			continue
		}

		res := tgram.SendMessageRequest{Text: message, ChatID: int64(home.UserID)}
		res.AddKeyboardElementRow([]tgram.InlineKeyboardElement{
			{Text: "⏰ Check hourly forecast", CallbackData: fmt.Sprintf("hourly:%f,%f", home.Latitude, home.Longitude)},
			{Text: "📆 Check daily forecast", CallbackData: fmt.Sprintf("daily:%f,%f", home.Latitude, home.Longitude)},
		})

		err = p.telegram.SendMessage(res)
		if err != nil {
			logger.Error("failed to send rainy update to telegram", "error", err.Error())
			continue
		}
	}

	return nil
}

func FindNextRainyDay(forecasts []*weather.Forecast) *weather.Forecast {
	for _, f := range forecasts {
		if f.IsRainy() {
			// We only want to get today if it's early morning. If we're
			// checking after lunch, might as well check upcoming days.
			if isToday(f.DateTimeTS) && isNowPastLunchTime() {
				continue
			}

			return f
		}
	}

	return nil
}

func FindNextHighTemperature(forecasts []*weather.Forecast) *weather.Forecast {
	if len(forecasts) == 0 {
		return nil
	}

	for _, f := range forecasts[:len(forecasts)-1] {
		if f.MaximumTemperature > 32 {
			return f
		}
	}

	return nil
}

func FindNextLowTemperature(forecasts []*weather.Forecast) *weather.Forecast {
	if len(forecasts) == 0 {
		return nil
	}

	for _, f := range forecasts[:len(forecasts)-1] {
		if f.MinimumTemperature < 10 {
			return f
		}
	}

	return nil
}

func isToday(unix int) bool {
	t := time.Unix(int64(unix), 0)
	return t.Day() == time.Now().Day()
}

func isNowPastLunchTime() bool {
	return time.Now().Hour() > 15
}
