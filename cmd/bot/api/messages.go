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
	locations      location.Repository
}

func NewMessageController(l location.Client, w weather.Client, c *conversation.ConvoRepository, ll location.Repository) *MessageController {
	return &MessageController{l, w, c, ll}
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

func (g *MessageController) ProcessHomeCommand(ctx context.Context, p *tgram.WebhookRequest) string {
	query := tgram.ExtractCommandQuery(p.Message.Text)
	if len(query) == 0 {
		_, err := g.convos.AddQuestion(ctx, fmt.Sprint(p.GetFromID()), "AWAITING_HOME")
		if err != nil {
			slog.Error("add question", "error", err.Error())
			return msg.MsgUnableToGetReport
		}

		home, err := g.locations.GetHome(ctx, p.GetFromID())
		if err != nil {
			slog.Error("query home", "error", err.Error())
			return msg.MsgHomeQuestion
		}

		if home == nil {
			return msg.MsgHomeQuestion
		}

		return fmt.Sprintf("Your current home is %s\\. %s", home.Name, msg.MsgHomeQuestion)
	}

	if convo, err := g.convos.Find(ctx, fmt.Sprint(p.GetFromID())); err == nil && convo != nil && !convo.Answered {
		err = g.convos.MarkQuestionAnswered(ctx, fmt.Sprint(p.GetFromID()))
		if err != nil {
			slog.Error("unable to mark question as answered", "error", err.Error())
			return msg.MsgUnableToGetReport
		}
	}

	return g.setHome(ctx, p, query)
}

func (g *MessageController) setHome(ctx context.Context, p *tgram.WebhookRequest, locationName string) string {
	location, err := g.locations.GetLocation(ctx, locationName)
	if err != nil {
		slog.Error("query location by name", "error", err.Error())
		return msg.MsgUnableToGetReport
	}

	// If the location doesn't exist, we create it
	if location == nil {
		location, err = g.locations.CreateLocation(ctx, locationName)
		if err != nil {
			slog.Error("create location", "error", err.Error())
			return msg.MsgUnableToGetReport
		}
	}

	// if it just has the name, we hydrate the database.
	if location.Latitude == 0 || location.Longitude == 0 {
		remote, err := g.locationClient.FindLocation(locationName)
		if err != nil {
			slog.Error("find location in third party", "error", err.Error())
			return msg.MsgUnableToGetReport
		}

		location.Latitude = remote.Latitude
		location.Longitude = remote.Longitude
		location.Country = remote.Country
		location.CountryCode = remote.CountryCode

		err = g.locations.UpdateLocation(ctx, location)
		if err != nil {
			slog.Error("update location", "error", err.Error())
			return msg.MsgUnableToGetReport
		}
	}

	err = g.locations.SetHome(ctx, p.GetFromID(), location)
	if err != nil {
		slog.Error("set home", "error", err.Error())
		return msg.MsgUnableToGetReport
	}

	return fmt.Sprintf(`Successfully set %s as your home! I'll now watch it for any weather changes and let you know 🙂`, locationName)
}

func (g *MessageController) ProcessNonCommand(ctx context.Context, p *tgram.WebhookRequest) string {
	convo, err := g.convos.Find(ctx, fmt.Sprint(p.GetFromID()))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		panic(err)
	} else if errors.Is(err, sql.ErrNoRows) || (convo != nil && convo.Answered) {
		return msg.MsgUnknownText
	}

	if convo.LastQuestionAsked == "AWAITING_HOME" {
		return g.setHome(ctx, p, p.Message.Text)
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
