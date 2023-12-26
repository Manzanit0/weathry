package api

import (
	"strconv"
	"strings"

	"log/slog"

	"github.com/manzanit0/weathry/cmd/bot/msg"
	"github.com/manzanit0/weathry/cmd/bot/services"
	"github.com/manzanit0/weathry/pkg/geocode"
	"github.com/manzanit0/weathry/pkg/tgram"
	"github.com/manzanit0/weathry/pkg/weather"
)

type CallbackController struct {
	weatherService *services.WeatherService
}

func NewCallbackController(l geocode.Client, w weather.Client) *CallbackController {
	srv := services.NewWeatherService(l, w)
	return &CallbackController{weatherService: srv}
}

func (g *CallbackController) ProcessCallbackQuery(p *tgram.WebhookRequest) string {
	s := strings.Split(p.CallbackQuery.Data, ":")
	if len(s) != 2 {
		slog.Error("unexpected callback query data format", "callback_data", p.CallbackQuery.Data, "error", "expected format: hourly:lat,lon")
		return msg.MsgUnexpectedError
	}

	ss := strings.Split(s[1], ",")
	if len(s) != 2 {
		slog.Error("unexpected callback query data format", "callback_data", p.CallbackQuery.Data, "error", "expected format: hourly:lat,lon")
		return msg.MsgUnexpectedError
	}

	lat, err := strconv.ParseFloat(ss[0], 64)
	if err != nil {
		slog.Error("invalid latitude format", "error", err.Error(), "callback_data", p.CallbackQuery.Data)
		return msg.MsgUnexpectedError
	}

	lon, err := strconv.ParseFloat(ss[1], 64)
	if err != nil {
		slog.Error("invalid longitude format", "error", err.Error(), "callback_data", p.CallbackQuery.Data)
		return msg.MsgUnexpectedError
	}

	switch s[0] {
	case "hourly":
		message, err := g.weatherService.GetHourlyWeatherByCoordinates(lat, lon)
		if err != nil {
			slog.Error("get hourly weather", "error", err.Error())
			return msg.MsgUnableToGetReport
		}

		return message
	case "daily":
		message, err := g.weatherService.GetDailyWeatherByCoordinates(lat, lon)
		if err != nil {
			slog.Error("get daily weather", "error", err.Error())
			return msg.MsgUnableToGetReport
		}

		return message
	default:
		slog.Error("unreachable line reached")
		return msg.MsgUnexpectedError
	}
}
