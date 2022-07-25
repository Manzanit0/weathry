package pings

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/manzanit0/weathry/pkg/location"
	"github.com/manzanit0/weathry/pkg/tgram"
	"github.com/manzanit0/weathry/pkg/weather"
)

// Madrid
const lat, lon = 40.2085, -3.713

// London
// const lat2, lon2 = 51.5285582, -0.2416811

type Pinger interface {
	MonitorWeather(context.Context) error
}

func NewBackgroundPinger(w weather.Client, l location.Client, t tgram.Client) Pinger {
	return &backgroundPinger{w: w, l: l, t: t}
}

type backgroundPinger struct {
	w weather.Client
	l location.Client
	t tgram.Client
}

func (p *backgroundPinger) MonitorWeather(ctx context.Context) error {
	ticker := time.NewTicker(time.Minute)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			h, m, _ := time.Now().Clock()
			// FIXME: deploys may fuck up this mechanic: if a deploy happens
			// exactly at h:00... might miss the message.
			if m == 0 && (h == 19 || h == 8) {
				err := p.PingRainyForecasts()
				if err != nil {
					log.Print(err.Error())
				}
			}
		}
	}
}

func (p *backgroundPinger) PingRainyForecasts() error {
	forecasts, err := p.w.GetUpcomingWeather(lat, lon)
	if err != nil {
		return fmt.Errorf("error requesting upcoming weather: %w", err)
	}

	forecast := FindNextRainyDay(forecasts)
	if forecast != nil {
		var message string
		if isToday(forecast.DateTimeTS) {
			message = "Heads up, it's going to be raining today!"
		} else {
			message = fmt.Sprintf("It will be raining next %s.", forecast.FormattedDateTime())
		}

		chatID, err := getChatIDFromEnv()
		if err != nil {
			return fmt.Errorf("unexpected error getting chat_id from environment: %w", err)
		}

		err = p.t.SendMessage(tgram.SendMessageRequest{Text: message, ChatID: chatID})
		if err != nil {
			return fmt.Errorf("failed to send rainy update to telegram: %w", err)
		}
	}

	forecast = FindNextHighTemperature(forecasts)
	if forecast != nil {
		var message string
		if isToday(forecast.DateTimeTS) {
			message = fmt.Sprintf("Heads up, going to be pretty hot with a max of %.2fºC! 🔥", forecast.MaximumTemperature)
		} else {
			message = fmt.Sprintf("Next %s temperatures are going to rise all the way to %.2fºC! 🔥", forecast.FormattedDateTime(), forecast.MaximumTemperature)
		}

		chatID, err := getChatIDFromEnv()
		if err != nil {
			return fmt.Errorf("unexpected error getting chat_id from environment: %w", err)
		}

		err = p.t.SendMessage(tgram.SendMessageRequest{Text: message, ChatID: chatID})
		if err != nil {
			return fmt.Errorf("failed to send rainy update to telegram: %w", err)
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

	previousForecast := forecasts[0]
	for _, f := range forecasts[:len(forecasts)-1] {
		if f.MaximumTemperature > 32 && f.MaximumTemperature > previousForecast.MaximumTemperature {
			return f
		}

		previousForecast = f
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

func getChatIDFromEnv() (int64, error) {
	var chatID string
	if chatID = os.Getenv("MY_TELEGRAM_CHAT_ID"); chatID == "" {
		return 0, fmt.Errorf("failed get chat ID from MY_TELEGRAM_CHAT_ID OS enviroment variable")
	}

	chatIDint, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse MY_TELEGRAM_CHAT_ID as integer: %s", err.Error())
	}

	return chatIDint, nil
}
