package api

import (
	"context"
	"database/sql"
	"errors"
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

	return fmt.Sprintf(`Successfully set %s as your home! I'll now watch it for any weather changes and let you know 🙂`, locationName)
}

func (g *MessageController) ProcessNonCommand(ctx context.Context, p *tgram.WebhookRequest) string {
	convo, err := g.convos.Find(ctx, fmt.Sprint(p.GetFromID()))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return msg.MsgUnexpectedError
	} else if errors.Is(err, sql.ErrNoRows) || (convo != nil && convo.Answered) {
		return msg.MsgUnknownText
	}

	if convo.LastQuestionAsked == conversation.QuestionHome {
		return g.setHome(ctx, p, p.Message.Text)
	}

	message, err := g.forecastFromQuestion(convo.LastQuestionAsked, p.Message.Text)
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

func (g *MessageController) forecastFromQuestion(question, response string) (string, error) {
	switch question {
	case conversation.QuestionHourlyWeather:
		return g.forecaster.GetHourlyWeatherByLocationName(response)
	case conversation.QuestionDailyWeather:
		return g.forecaster.GetDailyWeatherByLocationName(response)
	default:
		return "hey!", nil
	}
}
