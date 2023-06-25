package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/manzanit0/weathry/cmd/bot/action"
	"github.com/manzanit0/weathry/cmd/bot/conversation"
	"github.com/manzanit0/weathry/cmd/bot/msg"
	"github.com/manzanit0/weathry/pkg/location"
	"github.com/manzanit0/weathry/pkg/tgram"
	"github.com/manzanit0/weathry/pkg/weather"
	"golang.org/x/exp/slog"
)

type MessageController struct {
	locationClient location.Client
	weatherClient  weather.Client
	convos         *conversation.ConvoRepository
}

func NewMessageController(l location.Client, w weather.Client, c *conversation.ConvoRepository) *MessageController {
	return &MessageController{l, w, c}
}

func (g *MessageController) ProcessDailyCommand(ctx context.Context, p *tgram.WebhookRequest) string {
	query := tgram.ExtractCommandQuery(p.Message.Text)
	if len(query) == 0 {
		_, err := g.convos.AddQuestion(ctx, fmt.Sprint(p.GetFromID()), "AWAITING_DAILY_WEATHER_CITY")
		if err != nil {
			panic(err)
		}

		return msg.MsgLocationQuestionWeek
	}

	if convo, err := g.convos.Find(ctx, fmt.Sprint(p.GetFromID())); err == nil && convo != nil && !convo.Answered {
		err = g.convos.MarkQuestionAnswered(ctx, fmt.Sprint(p.GetFromID()))
		if err != nil {
			slog.Error("unable to mark question as answered", "error", err.Error())
		}
	}

	message, err := action.GetDailyWeather(g.locationClient, g.weatherClient, query)
	if err != nil {
		slog.Error("get upcoming weather", "error", err.Error())
		return msg.MsgUnableToGetReport
	}

	return message
}

func (g *MessageController) ProcessHourlyCommand(ctx context.Context, p *tgram.WebhookRequest) string {
	query := tgram.ExtractCommandQuery(p.Message.Text)
	if len(query) == 0 {
		_, err := g.convos.AddQuestion(ctx, fmt.Sprint(p.GetFromID()), "AWAITING_HOURLY_WEATHER_CITY")
		if err != nil {
			panic(err)
		}

		return msg.MsgLocationQuestionDay
	}

	if convo, err := g.convos.Find(ctx, fmt.Sprint(p.GetFromID())); err == nil && convo != nil && !convo.Answered {
		err = g.convos.MarkQuestionAnswered(ctx, fmt.Sprint(p.GetFromID()))
		if err != nil {
			slog.Error("unable to mark question as answered", "error", err.Error())
		}
	}

	message, err := action.GetHourlyWeatherByLocationName(g.locationClient, g.weatherClient, query)
	if err != nil {
		slog.Error("get upcoming weather", "error", err.Error())
		return msg.MsgUnableToGetReport
	}

	return message
}

func (g *MessageController) ProcessNonCommand(ctx context.Context, p *tgram.WebhookRequest) string {
	convo, err := g.convos.Find(ctx, fmt.Sprint(p.GetFromID()))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		panic(err)
	} else if errors.Is(err, sql.ErrNoRows) || (convo != nil && convo.Answered) {
		return msg.MsgUnknownText
	}

	message, err := forecastFromQuestion(g.locationClient, g.weatherClient, convo.LastQuestionAsked, p.Message.Text)
	if err != nil {
		slog.Error("get forecast from question", "error", err.Error())
		return msg.MsgUnableToGetReport
	}

	err = g.convos.MarkQuestionAnswered(ctx, fmt.Sprint(p.GetFromID()))
	if err != nil {
		slog.Error("unable to mark question as answered", "error", err.Error())
	}

	return message
}

func forecastFromQuestion(locClient location.Client, weatherClient weather.Client, question, response string) (string, error) {
	switch question {
	case "AWAITING_HOURLY_WEATHER_CITY":
		return action.GetHourlyWeatherByLocationName(locClient, weatherClient, response)
	case "AWAITING_DAILY_WEATHER_CITY":
		return action.GetDailyWeather(locClient, weatherClient, response)
	default:
		return "hey!", nil
	}
}
