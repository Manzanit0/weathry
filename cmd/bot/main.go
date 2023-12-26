package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v4/stdlib"

	"github.com/manzanit0/weathry/cmd/bot/api"
	"github.com/manzanit0/weathry/cmd/bot/conversation"
	"github.com/manzanit0/weathry/cmd/bot/location"
	"github.com/manzanit0/weathry/cmd/bot/msg"
	"github.com/manzanit0/weathry/cmd/bot/users"
	"github.com/manzanit0/weathry/pkg/env"
	"github.com/manzanit0/weathry/pkg/geocode"
	"github.com/manzanit0/weathry/pkg/logger"
	"github.com/manzanit0/weathry/pkg/middleware"
	"github.com/manzanit0/weathry/pkg/tgram"
	"github.com/manzanit0/weathry/pkg/weather"
	"github.com/manzanit0/weathry/pkg/whttp"
)

const ServiceName = "bot"

func init() {
	logger.InitGlobalSlog(ServiceName)
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

	geocoder, err := newGeocoder()
	if err != nil {
		panic(err)
	}

	errorTgramClient, err := env.NewErroryTgramClient()
	if err != nil {
		panic(err)
	}

	myTelegramChatID, err := env.MyTelegramChatID()
	if err != nil {
		panic(err)
	}

	usersClient := users.NewDBClient(db)

	r := gin.New()
	r.Use(middleware.TraceID())
	r.Use(middleware.Recovery(errorTgramClient, myTelegramChatID))
	r.Use(middleware.Logger(false))

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.Use(middleware.TelegramAuth(usersClient))
	r.POST("/telegram/webhook", telegramWebhookController(geocoder, owmClient, &convos, locations))

	// background job to ping users on weather changes
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
	geocoder geocode.Client,
	weatherClient weather.Client,
	convos *conversation.ConvoRepository,
	locations location.Repository,
) func(c *gin.Context) {
	callbackCtrl := api.NewCallbackController(geocoder, weatherClient)
	messageCtrl := api.NewMessageController(geocoder, weatherClient, convos, locations)
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

		ctx := c.Request.Context()

		if query := tgram.ExtractCommandQuery(p.Message.Text); len(query) == 0 {
			question, prompt := getQuestionAndPrompt(p.Message.Text)
			if question != "" {
				_, err := convos.AddQuestion(ctx, fmt.Sprint(p.GetFromID()), question)
				if err != nil {
					c.JSON(200, webhookResponse(p, msg.MsgUnexpectedError))
					return
				}

				c.JSON(200, webhookResponse(p, prompt))
				return
			}
		}

		var message string

		switch {
		case strings.HasPrefix(p.Message.Text, "/daily"):
			message = messageCtrl.ProcessDailyCommand(ctx, p)

		case strings.HasPrefix(p.Message.Text, "/hourly"):
			message = messageCtrl.ProcessHourlyCommand(ctx, p)

		case strings.HasPrefix(p.Message.Text, "/home"):
			message = messageCtrl.ProcessHomeCommand(ctx, p)

		case strings.HasPrefix(p.Message.Text, "/help"):
			message = fmt.Sprintf(msg.MsgHelp, p.GetFromFirstName())

		default:
			message = messageCtrl.ProcessNonCommand(ctx, p)
		}

		c.JSON(200, webhookResponse(p, message))
	}
}

func getQuestionAndPrompt(text string) (string, string) {
	switch text {
	case "/daily":
		return conversation.QuestionDailyWeather, msg.MsgLocationQuestionDay
	case "/hourly":
		return conversation.QuestionHourlyWeather, msg.MsgLocationQuestionWeek
	case "/home":
		return conversation.QuestionHome, msg.MsgHomeQuestion
	default:
		return "", ""
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

func newGeocoder() (geocode.Client, error) {
	return geocode.NewOpenstreetmapClient(), nil
}
