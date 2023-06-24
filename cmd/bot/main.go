package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/manzanit0/weathry/cmd/bot/msg"
	"github.com/manzanit0/weathry/pkg/env"
	"github.com/manzanit0/weathry/pkg/location"
	"github.com/manzanit0/weathry/pkg/middleware"
	"github.com/manzanit0/weathry/pkg/pings"
	"github.com/manzanit0/weathry/pkg/tgram"
	"github.com/manzanit0/weathry/pkg/weather"
	"github.com/manzanit0/weathry/pkg/whttp"
	"golang.org/x/exp/slog"

	_ "github.com/jackc/pgx/v4/stdlib"
)

func init() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
}

const CtxKeyPayload = "gin.ctx.payload"

func main() {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(fmt.Errorf("unable to open db conn: %w", err))
	}

	defer func() {
		err = db.Close()
		if err != nil {
			slog.Error("error closing db connection", "error", err.Error())
		}
	}()

	if err := db.Ping(); err != nil {
		panic(fmt.Errorf("unable to ping database: %w", err))
	} else {
		slog.Info("connected to the database successfully")
	}

	convos := ConvoRepository{db}

	owmClient, err := newWeatherClient()
	if err != nil {
		panic(err)
	}

	psClient, err := newLocationClient()
	if err != nil {
		panic(err)
	}

	tgramClient, err := newTelegramClient()
	if err != nil {
		panic(err)
	}

	errorTgramClient, err := env.NewErroryTgramClient()
	if err != nil {
		panic(err)
	}

	usersClient, err := newUsersClient()
	if err != nil {
		panic(err)
	}

	myTelegramChatID, err := env.MyTelegramChatID()
	if err != nil {
		panic(err)
	}

	r := gin.New()
	r.Use(middleware.Recovery(errorTgramClient, myTelegramChatID))
	r.Use(middleware.Logging())

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.Use(TelegramAuth(usersClient))
	r.POST("/telegram/webhook", telegramWebhookController(psClient, owmClient, &convos))

	// background job to ping users on weather changes
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pingDone := make(chan struct{})

	go func() {
		defer close(pingDone)
		slog.Info("starting pinger")

		pinger := pings.NewBackgroundPinger(owmClient, psClient, tgramClient)
		if err := pinger.MonitorWeather(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				slog.Info("pinger shutdown gracefully")
				return
			}

			slog.Error("pinger shutdown abruptly", "error", err.Error())
			stop()
		}
	}()

	var port string
	if port = os.Getenv("PORT"); port == "" {
		port = "8080"
	}

	srv := &http.Server{Addr: fmt.Sprintf(":%s", port), Handler: r}
	go func() {
		slog.Info(fmt.Sprintf("serving HTTP on :%s", port))

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server shutdown abruptly", "error", err.Error())
		} else {
			slog.Info("server shutdown gracefully")
		}

		stop()
	}()

	// Listen for OS interrupt
	<-ctx.Done()
	stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err.Error())
	}

	slog.Info("server exited")

	<-pingDone
	slog.Info("pinger exited")
}

// @see https://core.telegram.org/bots/api#markdownv2-style
func webhookResponse(p *tgram.WebhookRequest, text string) gin.H {
	return gin.H{
		"method":     "sendMessage",
		"chat_id":    p.GetFromID(),
		"text":       text,
		"parse_mode": "MarkdownV2",
	}
}

func TelegramAuth(usersClient UsersClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var r tgram.WebhookRequest
		if err := c.ShouldBindJSON(&r); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Errorf("payload does not conform with telegram contract: %w", err).Error(),
			})
			return
		}

		if !strings.EqualFold(r.GetFromUsername(), "manzanit0") {
			slog.Info("unauthorised user", "username", r.GetFromUsername())
			c.JSON(http.StatusUnauthorized, gin.H{})
			return
		}

		c.Set(CtxKeyPayload, &r)

		username := r.GetFromUsername()
		firstName := r.GetFromFirstName()
		lastName := r.GetFromLastName()
		err := usersClient.CreateUser(c.Request.Context(), CreateUserPayload{
			ID:           fmt.Sprint(r.GetFromID()),
			Username:     &username,
			FirstName:    &firstName,
			LastName:     &lastName,
			LanguageCode: r.GetFromLanguageCode(),
		})
		if err != nil {
			slog.Error("unable to track user", "error", err.Error(), "username", username)
		} else {
			slog.Info("user tracked", "username", username)
		}

		c.Next()
	}
}

func telegramWebhookController(locClient location.Client, weatherClient weather.Client, convos *ConvoRepository) func(c *gin.Context) {
	return func(c *gin.Context) {
		var p *tgram.WebhookRequest

		if i, ok := c.Get(CtxKeyPayload); ok {
			p = i.(*tgram.WebhookRequest)
		} else {
			c.JSON(400, gin.H{"error": "bad request"})
			return
		}

		if p.Message == nil {
			c.JSON(200, webhookResponse(p, msg.MsgUnsupportedInteraction))
			return
		}

		switch {
		case strings.HasPrefix(p.Message.Text, "/daily"):
			query := getQuery(p.Message.Text)
			if len(query) == 0 {
				_, err := convos.AddQuestion(c.Request.Context(), fmt.Sprint(p.GetFromID()), "AWAITING_DAILY_WEATHER_CITY")
				if err != nil {
					panic(err)
				}

				c.JSON(200, webhookResponse(p, msg.MsgLocationQuestionWeek))
				return
			}

			if convo, err := convos.Find(c.Request.Context(), fmt.Sprint(p.GetFromID())); err == nil && convo != nil && !convo.Answered {
				err = convos.MarkQuestionAnswered(c.Request.Context(), fmt.Sprint(p.GetFromID()))
				if err != nil {
					slog.Error("unable to mark question as answered", "error", err.Error())
				}
			}

			message, err := GetDailyWeather(locClient, weatherClient, query)
			if err != nil {
				slog.Error("get upcoming weather", "error", err.Error())
				c.JSON(200, webhookResponse(p, msg.MsgUnableToGetReport))
				return
			}

			c.JSON(200, webhookResponse(p, message))
		case strings.HasPrefix(p.Message.Text, "/hourly"):
			query := getQuery(p.Message.Text)
			if len(query) == 0 {
				_, err := convos.AddQuestion(c.Request.Context(), fmt.Sprint(p.GetFromID()), "AWAITING_HOURLY_WEATHER_CITY")
				if err != nil {
					panic(err)
				}

				c.JSON(200, webhookResponse(p, msg.MsgLocationQuestionDay))
				return
			}

			if convo, err := convos.Find(c.Request.Context(), fmt.Sprint(p.GetFromID())); err == nil && convo != nil && !convo.Answered {
				err = convos.MarkQuestionAnswered(c.Request.Context(), fmt.Sprint(p.GetFromID()))
				if err != nil {
					slog.Error("unable to mark question as answered", "error", err.Error())
				}
			}

			message, err := GetHourlyWeather(locClient, weatherClient, query)
			if err != nil {
				slog.Error("get upcoming weather", "error", err.Error())
				c.JSON(200, webhookResponse(p, msg.MsgUnableToGetReport))
				return
			}

			c.JSON(200, webhookResponse(p, message))
		default:
			convo, err := convos.Find(c.Request.Context(), fmt.Sprint(p.GetFromID()))
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				panic(err)
			} else if errors.Is(err, sql.ErrNoRows) || (convo != nil && convo.Answered) {
				c.JSON(200, webhookResponse(p, msg.MsgUnknownText))
				return
			}

			message, err := forecastFromQuestion(locClient, weatherClient, convo.LastQuestionAsked, p.Message.Text)
			if err != nil {
				slog.Error("get forecast from question", "error", err.Error())
				c.JSON(200, webhookResponse(p, msg.MsgUnableToGetReport))
				return
			}

			err = convos.MarkQuestionAnswered(c.Request.Context(), fmt.Sprint(p.GetFromID()))
			if err != nil {
				slog.Error("unable to mark question as answered", "error", err.Error())
			}

			c.JSON(200, webhookResponse(p, message))
		}
	}
}

func forecastFromQuestion(locClient location.Client, weatherClient weather.Client, question, response string) (string, error) {
	switch question {
	case "AWAITING_HOURLY_WEATHER_CITY":
		return GetHourlyWeather(locClient, weatherClient, response)
	case "AWAITING_DAILY_WEATHER_CITY":
		return GetDailyWeather(locClient, weatherClient, response)
	default:
		return "hey!", nil
	}
}

func GetDailyWeather(locClient location.Client, weatherClient weather.Client, query string) (string, error) {
	location, err := locClient.FindLocation(query)
	if err != nil {
		return "", fmt.Errorf("find location: %w", err)
	}

	forecasts, err := weatherClient.GetUpcomingWeather(location.Latitude, location.Longitude)
	if err != nil {
		return "", fmt.Errorf("get weather: %w", err)
	}

	return msg.NewForecastTableMessage(location, forecasts, msg.WithTemperatureDiff()), nil
}

func GetHourlyWeather(locClient location.Client, weatherClient weather.Client, query string) (string, error) {
	location, err := locClient.FindLocation(query)
	if err != nil {
		return "", fmt.Errorf("find location: %w", err)
	}

	forecasts, err := weatherClient.GetHourlyForecast(location.Latitude, location.Longitude)
	if err != nil {
		return "", fmt.Errorf("get weather: %w", err)
	}

	// Just 9 forecasts for the hourly, to cover 24h.
	if len(forecasts) > 9 {
		filtered := make([]*weather.Forecast, 9)
		for i := 0; i < 9; i++ {
			filtered[i] = forecasts[i]
		}

		return msg.NewForecastTableMessage(location, filtered, msg.WithTime()), nil
	}

	return msg.NewForecastTableMessage(location, forecasts, msg.WithTime()), nil
}

func getQuery(text string) string {
	strs := strings.Split(text, " ")
	if len(strs) < 2 {
		return text
	}

	return strings.Join(strs[1:], " ")
}

func newWeatherClient() (weather.Client, error) {
	var openWeatherMapAPIKey string
	if openWeatherMapAPIKey = os.Getenv("OPENWEATHERMAP_API_KEY"); openWeatherMapAPIKey == "" {
		return nil, fmt.Errorf("missing OPENWEATHERMAP_API_KEY environment variable. Please check your environment.")
	}

	httpClient := whttp.NewLoggingClient()
	return weather.NewOpenWeatherMapClient(httpClient, openWeatherMapAPIKey), nil
}

func newLocationClient() (location.Client, error) {
	var positionStackAPIKey string
	if positionStackAPIKey = os.Getenv("POSITIONSTACK_API_KEY"); positionStackAPIKey == "" {
		return nil, fmt.Errorf("missing POSITIONSTACK_API_KEY environment variable. Please check your environment.")
	}

	httpClient := whttp.NewLoggingClient()
	return location.NewPositionStackClient(httpClient, positionStackAPIKey), nil
}

func newTelegramClient() (tgram.Client, error) {
	var telegramBotToken string
	if telegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN"); telegramBotToken == "" {
		return nil, fmt.Errorf("missing TELEGRAM_BOT_TOKEN environment variable. Please check your environment.")
	}

	httpClient := whttp.NewLoggingClient()
	return tgram.NewClient(httpClient, telegramBotToken), nil
}

func newUsersClient() (UsersClient, error) {
	var host string
	if host = os.Getenv("USER_SERVICE_HOST"); host == "" {
		return nil, fmt.Errorf("missing USER_SERVICE_HOST environment variable. Please check your environment.")
	}

	return NewUsersClient(host), nil
}

type UsersClient interface {
	CreateUser(context.Context, CreateUserPayload) error
}

type CreateUserPayload struct {
	ID           string
	Username     *string `json:"username"`
	FirstName    *string `json:"first_name"`
	LastName     *string `json:"last_name"`
	LanguageCode string  `json:"language_code"`
	IsBot        string  `json:"is_bot"`
}

type usersClient struct {
	host string
	h    *http.Client
}

func NewUsersClient(host string) UsersClient {
	h := &http.Client{Timeout: 10 * time.Second}
	return &usersClient{host: host, h: h}
}

func (c *usersClient) CreateUser(ctx context.Context, req CreateUserPayload) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}

	body := bytes.NewBuffer(b)
	endpoint := fmt.Sprintf("%s/users/%s", c.host, req.ID)
	r, err := http.NewRequest(http.MethodPut, endpoint, body)
	if err != nil {
		return err
	}

	resp, err := c.h.Do(r)
	if err != nil {
		return fmt.Errorf("failed to do POST to %s: %s", endpoint, err.Error())
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		responseBody := &bytes.Buffer{}
		_, err := responseBody.ReadFrom(resp.Body)
		if err != nil {
			return err
		}

		return fmt.Errorf("unexpected response: (%d) %s", resp.StatusCode, responseBody.String())
	}

	return nil
}
