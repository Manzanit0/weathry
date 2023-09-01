package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/manzanit0/weathry/cmd/bot/api"
	"github.com/manzanit0/weathry/cmd/bot/conversation"
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
	logger = logger.With("service", "bot")
	slog.SetDefault(logger)
}

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

	convos := conversation.ConvoRepository{DB: db}

	locations := location.NewPgRepository(db)

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

	// To avoid other bots, or even actual humans, using my API quotas, just
	// authorised the users you want to use the BOT.
	authorisedUsers := strings.Split(os.Getenv("TELEGRAM_AUTHORISED_USERS"), ",")
	r.Use(middleware.TelegramAuth(usersClient, authorisedUsers...))
	r.POST("/telegram/webhook", telegramWebhookController(psClient, owmClient, &convos, locations))

	// background job to ping users on weather changes
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pingDone := make(chan struct{})

	go func() {
		defer middleware.Recover(errorTgramClient, myTelegramChatID)
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

func telegramWebhookController(
	locClient location.Client,
	weatherClient weather.Client,
	convos *conversation.ConvoRepository,
	locations location.Repository,
) func(c *gin.Context) {
	callbackCtrl := api.NewCallbackController(locClient, weatherClient)
	messageCtrl := api.NewMessageController(locClient, weatherClient, convos, locations)
	return func(c *gin.Context) {
		var p *tgram.WebhookRequest

		if i, ok := c.Get(middleware.CtxKeyPayload); ok {
			p = i.(*tgram.WebhookRequest)
		} else {
			c.JSON(400, gin.H{"error": "bad request"})
			return
		}

		if p.IsCallbackQuery() {
			message := callbackCtrl.ProcessCallbackQuery(p)
			c.JSON(200, webhookResponse(p, message))
			return
		}

		if p.Message == nil {
			c.JSON(200, webhookResponse(p, msg.MsgUnsupportedInteraction))
			return
		}

		if query := tgram.ExtractCommandQuery(p.Message.Text); len(query) == 0 {
			question, prompt := getQuestionAndPrompt(p.Message.Text)
			_, err := convos.AddQuestion(c.Request.Context(), fmt.Sprint(p.GetFromID()), question)
			if err != nil {
				c.JSON(200, webhookResponse(p, msg.MsgUnexpectedError))
				return
			}

			c.JSON(200, webhookResponse(p, prompt))
			return
		}

		switch {
		case strings.HasPrefix(p.Message.Text, "/daily"):
			message := messageCtrl.ProcessDailyCommand(c.Request.Context(), p)
			c.JSON(200, webhookResponse(p, message))
			return

		case strings.HasPrefix(p.Message.Text, "/hourly"):
			message := messageCtrl.ProcessHourlyCommand(c.Request.Context(), p)
			c.JSON(200, webhookResponse(p, message))
			return

		case strings.HasPrefix(p.Message.Text, "/home"):
			message := messageCtrl.ProcessHomeCommand(c.Request.Context(), p)
			c.JSON(200, webhookResponse(p, message))
			return

		default:
			message := messageCtrl.ProcessNonCommand(c.Request.Context(), p)
			c.JSON(200, webhookResponse(p, message))
			return
		}
	}
}

func getQuestionAndPrompt(cmd string) (string, string) {
	switch cmd {
	case "/daily":
		return conversation.QuestionHourlyWeather, msg.MsgLocationQuestionDay
	case "/hourly":
		return conversation.QuestionDailyWeather, msg.MsgLocationQuestionWeek
	case "/home":
		return conversation.QuestionHome, msg.MsgHomeQuestion
	default:
		return "", "'"
	}
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
	return location.NewOpenstreetmapClient(), nil
	// var positionStackAPIKey string
	// if positionStackAPIKey = os.Getenv("POSITIONSTACK_API_KEY"); positionStackAPIKey == "" {
	// 	return nil, fmt.Errorf("missing POSITIONSTACK_API_KEY environment variable. Please check your environment.")
	// }

	// httpClient := whttp.NewLoggingClient()
	// return location.NewPositionStackClient(httpClient, positionStackAPIKey), nil
}

func newTelegramClient() (tgram.Client, error) {
	var telegramBotToken string
	if telegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN"); telegramBotToken == "" {
		return nil, fmt.Errorf("missing TELEGRAM_BOT_TOKEN environment variable. Please check your environment.")
	}

	httpClient := whttp.NewLoggingClient()
	return tgram.NewClient(httpClient, telegramBotToken), nil
}

func newUsersClient() (middleware.UsersClient, error) {
	var host string
	if host = os.Getenv("USER_SERVICE_HOST"); host == "" {
		return nil, fmt.Errorf("missing USER_SERVICE_HOST environment variable. Please check your environment.")
	}

	return middleware.NewUsersClient(host), nil
}
