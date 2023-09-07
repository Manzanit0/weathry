package pings_test

import (
	"testing"
	"time"

	"github.com/manzanit0/weathry/pkg/pings"
	"github.com/manzanit0/weathry/pkg/weather"
)

func TestFindNextHighTemperature(t *testing.T) {
	testCases := []struct {
		desc      string
		forecasts []*weather.Forecast
		want      *weather.Forecast
	}{
		{
			desc:      "when nil forecasts are passed, it should return nil",
			want:      nil,
			forecasts: nil,
		},
		{
			desc:      "when no forcasts are passed, it should return nil",
			want:      nil,
			forecasts: []*weather.Forecast{},
		},
		{
			desc: "when the first forecast is above 32, it should be ignored",
			want: nil,
			forecasts: []*weather.Forecast{
				{MaximumTemperature: 35, DateTimeTS: int(time.Now().Unix())},
			},
		},
		{
			desc: "when the temperature increases from the previous day and is above 32, it should be returned",
			want: &weather.Forecast{MaximumTemperature: 33, MinimumTemperature: 23, DateTimeTS: int(time.Now().Add(48 * time.Hour).Unix())},
			forecasts: []*weather.Forecast{
				{MaximumTemperature: 12, DateTimeTS: int(time.Now().Unix())},
				{MaximumTemperature: 13, DateTimeTS: int(time.Now().Add(24 * time.Hour).Unix())},
				{MaximumTemperature: 33, DateTimeTS: int(time.Now().Add(48 * time.Hour).Unix())},
				{MaximumTemperature: 34, DateTimeTS: int(time.Now().Add(72 * time.Hour).Unix())},
			},
		},
		{
			desc: "when the temperature decreases from the previous day and is above 32, it should not be returned",
			want: nil,
			forecasts: []*weather.Forecast{
				{MaximumTemperature: 35, DateTimeTS: int(time.Now().Unix())},
				{MaximumTemperature: 34, DateTimeTS: int(time.Now().Add(24 * time.Hour).Unix())},
				{MaximumTemperature: 33, DateTimeTS: int(time.Now().Add(48 * time.Hour).Unix())},
				{MaximumTemperature: 32, DateTimeTS: int(time.Now().Add(72 * time.Hour).Unix())},
			},
		},
		{
			desc: "when the temperature increases and is above 32 immediately after the first forecast, it should be returned",
			want: &weather.Forecast{MaximumTemperature: 36, DateTimeTS: int(time.Now().Add(24 * time.Hour).Unix())},
			forecasts: []*weather.Forecast{
				{MaximumTemperature: 35, DateTimeTS: int(time.Now().Unix())},
				{MaximumTemperature: 36, DateTimeTS: int(time.Now().Add(24 * time.Hour).Unix())},
				{MaximumTemperature: 37, DateTimeTS: int(time.Now().Add(48 * time.Hour).Unix())},
				{MaximumTemperature: 38, DateTimeTS: int(time.Now().Add(72 * time.Hour).Unix())},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			got := pings.FindNextHighTemperature(tC.forecasts)
			if tC.want == nil && got != nil {
				t.Errorf("expected nil, got %v", got)
			}

			if tC.want != nil && got != nil && got.DateTimeTS != tC.want.DateTimeTS {
				t.Errorf("got %v, expected %v", got, tC.want)
			}
		})
	}
}
