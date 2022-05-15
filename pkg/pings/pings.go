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
			// FIXME: deploys may fuck up this mechanic: if a deploy happens exactly at hh:00... might miss the message.
			if m == 0 && (h == 19 || h == 8) {
				forecast, err := p.FindNextRainyDay()
				if err != nil {
					log.Printf("error checking rain: %s", err.Error())
					continue
				}

				if forecast != nil {
					var message string
					if isToday(forecast.DateTimeTS) {
						message = "Heads up, it's going to be raining today!"
					} else {
						message = fmt.Sprintf("It will be raining next %s.", forecast.FormattedDateTime())
					}

					// TODO: might want some alerting here and maybe end application?
					var chatID string
					if chatID = os.Getenv("MY_TELEGRAM_CHAT_ID"); chatID == "" {
						log.Printf("failed get chat ID from MY_TELEGRAM_CHAT_ID OS enviroment variable")
						continue
					}

					chatIDint, err := strconv.ParseInt(chatID, 10, 64)
					if err != nil {
						log.Printf("failed to parse MY_TELEGRAM_CHAT_ID as integer: %s", err.Error())
					}

					err = p.t.SendMessage(tgram.SendMessageRequest{
						Text:   message,
						ChatID: chatIDint,
					})
					if err != nil {
						log.Printf("failed to send rainy update to telegram: %s", err.Error())
					}
				}
			}
		}
	}
}

func (p *backgroundPinger) FindNextRainyDay() (*weather.Forecast, error) {
	forecasts, err := p.w.GetUpcomingWeather(lat, lon)
	if err != nil {
		return nil, err
	}

	for _, f := range forecasts {
		if f.IsRainy() {
			// We only want to get today if it's early morning. If we're
			// checking after lunch, might as well check upcoming days.
			if isTodayPastLunchTime(f.DateTimeTS) {
				continue
			}

			return f, nil
		}
	}

	return nil, nil
}

func isTodayPastLunchTime(unix int) bool {
	t := time.Unix(int64(unix), 0)
	return t.Day() == time.Now().Day() && t.Hour() >= 15
}

func isToday(unix int) bool {
	t := time.Unix(int64(unix), 0)
	return t.Day() == time.Now().Day()
}
