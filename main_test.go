package main

import (
	"strings"
	"testing"

	"github.com/manzanit0/weathry/pkg/weather"
)

func TestBuildMessage(t *testing.T) {
	testCases := []struct {
		desc      string
		forecasts []*weather.Forecast
		want      string
	}{
		{
			desc:      "when no forecasts are provided, a funny message is returned",
			forecasts: []*weather.Forecast{},
			want:      "hey, not sure why but I couldn't get any forecasts ¯\\_(ツ)_/¯",
		},
		{
			desc: "when multiple forecasts are provided, they're all included in the message",
			forecasts: []*weather.Forecast{
				{
					Condition:          "rain",
					Description:        "heavy rain",
					MinimumTemperature: -12.45,
					MaximumTemperature: 2.34,
					Humidity:           32,
					WindSpeed:          1.78,
					DateTimeTS:         1652439629,
					Location:           "Madrid, ES",
					Coordinates:        weather.Coordinates{Latitude: 23.23, Longitude: 32.32},
				},
				{
					Condition:          "cloudy",
					Description:        "mildly cloudy",
					MinimumTemperature: -8.78,
					MaximumTemperature: 7.12,
					Humidity:           21,
					WindSpeed:          3.92,
					DateTimeTS:         1652526029,
					Location:           "Madrid, ES",
					Coordinates:        weather.Coordinates{Latitude: 23.23, Longitude: 32.32},
				},
			},
			want: `Weather Report for Madrid, ES
- - - - - - - - - - - - - - - - - - - - - -
📅 Fri, 13 May 2022 13:00:29 CEST

TLDR:
🏷 heavy rain

Temperature:
❄️ -12.45°C
🔥 2.34ºC

Wind:
💨 1.78 m/s

Humidity:
💧 32%
- - - - - - - - - - - - - - - - - - - - - -
📅 Sat, 14 May 2022 13:00:29 CEST

TLDR:
🏷 mildly cloudy

Temperature:
❄️ -8.78°C
🔥 7.12ºC

Wind:
💨 3.92 m/s

Humidity:
💧 21%
- - - - - - - - - - - - - - - - - - - - - -`,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			got := BuildMessage(tC.forecasts)

			if !strings.EqualFold(got, tC.want) {
				t.Errorf("got:\n%s\nwant:\n%s", got, tC.want)
			}
		})
	}
}
