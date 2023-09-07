package api

import (
	"context"
	"fmt"

	"github.com/manzanit0/weathry/cmd/bot/conversation"
	"github.com/manzanit0/weathry/cmd/bot/location"
	"github.com/manzanit0/weathry/cmd/bot/msg"
	"github.com/manzanit0/weathry/cmd/bot/services"
	"github.com/manzanit0/weathry/pkg/geocode"
	"github.com/manzanit0/weathry/pkg/tgram"
	"github.com/manzanit0/weathry/pkg/weather"
	"golang.org/x/exp/slog"
)

type MessageController struct {
	geocoder   geocode.Client
	convos     *conversation.ConvoRepository
	locations  location.Repository
	forecaster *services.WeatherService
}

func NewMessageController(l geocode.Client, w weather.Client, c *conversation.ConvoRepository, ll location.Repository) *MessageController {
	s := services.NewWeatherService(l, w)
	return &MessageController{l, c, ll, s}
}

func (g *MessageController) ProcessDailyCommand(ctx context.Context, p *tgram.WebhookRequest) string {
	err := g.convos.MarkQuestionAnswered(ctx, fmt.Sprint(p.GetFromID()))
	if err != nil {
		slog.Error("unable to mark question as answered", "error", err.Error())
		return msg.MsgUnexpectedError
	}

	query := tgram.ExtractCommandQuery(p.Message.Text)
	message, err := g.forecaster.GetDailyWeatherByLocationName(query)
	if err != nil {
		slog.Error("get upcoming weather", "error", err.Error())
		return msg.MsgUnableToGetReport
	}

	return message
}

func (g *MessageController) ProcessHourlyCommand(ctx context.Context, p *tgram.WebhookRequest) string {
	err := g.convos.MarkQuestionAnswered(ctx, fmt.Sprint(p.GetFromID()))
	if err != nil {
		slog.Error("unable to mark question as answered", "error", err.Error())
		return msg.MsgUnexpectedError
	}

	query := tgram.ExtractCommandQuery(p.Message.Text)
	message, err := g.forecaster.GetHourlyWeatherByLocationName(query)
	if err != nil {
		slog.Error("get upcoming weather", "error", err.Error())
		return msg.MsgUnableToGetReport
	}

	return message
}

func (g *MessageController) ProcessHomeCommand(ctx context.Context, p *tgram.WebhookRequest) string {
	err := g.convos.MarkQuestionAnswered(ctx, fmt.Sprint(p.GetFromID()))
	if err != nil {
		slog.Error("unable to mark question as answered", "error", err.Error())
		return msg.MsgUnexpectedError
	}

	query := tgram.ExtractCommandQuery(p.Message.Text)
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
		remote, err := g.geocoder.Geocode(locationName)
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

	return fmt.Sprintf("Successfully set %s as your home\\! From now on, I\\'ll let you know of any relevant weather changes there 🙂", locationName)
}

func (g *MessageController) ProcessNonCommand(ctx context.Context, p *tgram.WebhookRequest) string {
	convo, err := g.convos.Find(ctx, fmt.Sprint(p.GetFromID()))

	switch {
	case err != nil:
		slog.Error("find conversation", "error", err.Error())
		return msg.MsgUnexpectedError

	case convo == nil:
		slog.Error("find conversation", "error", err.Error())
		return msg.MsgUnknownText

	case convo != nil && convo.Answered:
		return msg.MsgUnknownText

	case convo.LastQuestionAsked == conversation.QuestionHome:
		err = g.convos.MarkQuestionAnswered(ctx, fmt.Sprint(p.GetFromID()))
		if err != nil {
			slog.Error("unable to mark question as answered", "error", err.Error())
		}

		return g.setHome(ctx, p, p.Message.Text)

	case convo.LastQuestionAsked == conversation.QuestionHourlyWeather:
		err = g.convos.MarkQuestionAnswered(ctx, fmt.Sprint(p.GetFromID()))
		if err != nil {
			slog.Error("unable to mark question as answered", "error", err.Error())
		}

		message, err := g.forecaster.GetHourlyWeatherByLocationName(p.Message.Text)
		if err != nil {
			slog.Error("get forecast from question", "error", err.Error())
			return msg.MsgUnableToGetReport
		}

		return message

	case convo.LastQuestionAsked == conversation.QuestionDailyWeather:
		err = g.convos.MarkQuestionAnswered(ctx, fmt.Sprint(p.GetFromID()))
		if err != nil {
			slog.Error("unable to mark question as answered", "error", err.Error())
		}

		message, err := g.forecaster.GetDailyWeatherByLocationName(p.Message.Text)
		if err != nil {
			slog.Error("get forecast from question", "error", err.Error())
			return msg.MsgUnableToGetReport
		}

		return message

	default:
		return msg.MsgUnknownText
	}
}
