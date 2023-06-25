package api

import (
	"strconv"
	"strings"

	"github.com/manzanit0/weathry/cmd/bot/action"
	"github.com/manzanit0/weathry/cmd/bot/msg"
	"github.com/manzanit0/weathry/pkg/location"
	"github.com/manzanit0/weathry/pkg/tgram"
	"github.com/manzanit0/weathry/pkg/weather"
	"golang.org/x/exp/slog"
)

type CallbackController struct {
	locationClient location.Client
	weatherClient  weather.Client
}

func NewCallbackController(l location.Client, w weather.Client) *CallbackController {
	return &CallbackController{l, w}
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
		message, err := action.GetHourlyWeatherByCoordinates(g.locationClient, g.weatherClient, lat, lon)
		if err != nil {
			slog.Error("get hourly weather", "error", err.Error())
			return msg.MsgUnableToGetReport
		}

		return message
	case "daily":
		message, err := action.GetDailyWeatherByCoordinates(g.locationClient, g.weatherClient, lat, lon)
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
