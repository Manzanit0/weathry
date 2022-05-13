package pings

import (
	"context"
	"log"
	"time"

	"github.com/manzanit0/weathry/pkg/location"
	"github.com/manzanit0/weathry/pkg/weather"
)

// Madrid
const lat, lon = 40.2085, -3.713

// London
// const lat2, lon2 = 51.5285582, -0.2416811

type Pinger interface {
	MonitorWeather(context.Context) error
}

func NewBackgroundPinger(w weather.Client, l location.Client) Pinger {
	return &backgroundPinger{w: w, l: l}
}

type backgroundPinger struct {
	w weather.Client
	l location.Client
}

func (p *backgroundPinger) MonitorWeather(ctx context.Context) error {
	ticker := time.NewTicker(time.Minute)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			h, m, _ := time.Now().Clock()
			if m == 0 && h == 19 { // notify at 19.00h server time (UTC?)
				forecast, err := p.FindNextRainyDay()
				if err != nil {
					log.Printf("error checking rain: %s", err.Error())
					continue
				}

				if forecast != nil {
					log.Printf("it is rainy on %s!", forecast.FormattedDateTime())
				} else {
					log.Printf("the future looks sunny to me")
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
			return f, nil
		}
	}

	return nil, nil
}
