package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/manzanit0/weathry/pkg/location"
	"github.com/manzanit0/weathry/pkg/pings"
	"github.com/manzanit0/weathry/pkg/tgram"
	users "github.com/manzanit0/weathry/pkg/users/gen"
	"github.com/manzanit0/weathry/pkg/weather"
	"google.golang.org/grpc"
)

const CtxKeyPayload = "gin.ctx.payload"

func main() {
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

	usersClient, conn, err := newUsersClient()
	if err != nil {
		panic(err)
	}

	defer conn.Close()

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.Use(TelegramAuth(usersClient))
	r.POST("/telegram/webhook", telegramWebhookController(psClient, owmClient))

	// background job to ping users on weather changes
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pingDone := make(chan struct{})

	go func() {
		defer close(pingDone)
		log.Printf("starting pinger")

		pinger := pings.NewBackgroundPinger(owmClient, psClient, tgramClient)
		if err := pinger.MonitorWeather(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("pinger ended gracefully")
				return
			}

			log.Printf("pinger ended abruptly: %s", err.Error())
			stop()
		}
	}()

	var port string
	if port = os.Getenv("PORT"); port == "" {
		port = "8080"
	}

	srv := &http.Server{Addr: fmt.Sprintf(":%s", port), Handler: r}
	go func() {
		log.Printf("starting server")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server ended abruptly: %s", err.Error())
		} else {
			log.Printf("server ended gracefully")
		}

		stop()
	}()

	// Listen for OS interrupt
	<-ctx.Done()
	stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("server forced to shutdown: ", err)
	}

	log.Printf("server exited")

	<-pingDone
	log.Printf("pinger exited")
}

func webhookResponse(p *tgram.WebhookRequest, text string) gin.H {
	return gin.H{
		"method":  "sendMessage",
		"chat_id": p.Message.From.ID,
		"text":    text,
	}
}

func TelegramAuth(usersClient users.UsersClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var r tgram.WebhookRequest
		if err := c.ShouldBindJSON(&r); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Errorf("payload does not conform with telegram contract: %w", err).Error(),
			})
			return
		}

		if !strings.EqualFold(r.Message.Chat.Username, "manzanit0") {
			log.Printf("unauthorised user: %s", r.Message.Chat.Username)
			c.JSON(http.StatusUnauthorized, gin.H{})
			return
		}

		c.Set(CtxKeyPayload, &r)

		_, err := usersClient.Create(c.Request.Context(), &users.CreateRequest{
			TelegramChatId: int64(r.Message.From.ID),
			Username:       r.Message.From.Username,
			FirstName:      r.Message.From.FirstName,
			LastName:       r.Message.From.LastName,
			LanguageCode:   r.Message.From.LanguageCode,
		})
		if err != nil {
			log.Printf("unable to track user: %s", err.Error())
		}

		c.Next()
	}
}

func BuildMessage(f []*weather.Forecast) string {
	if len(f) == 0 {
		return "hey, not sure why but I couldn't get any forecasts ¯\\_(ツ)_/¯"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Weather Report for %s", f[0].Location))
	for _, v := range f {
		ts := v.FormattedDateTime()
		sb.WriteString(fmt.Sprintf(`
- - - - - - - - - - - - - - - - - - - - - -
📅 %s

TLDR:
🏷 %s

Temperature:
❄️ %0.2f°C
🔥 %0.2fºC

Wind:
💨 %0.2f m/s

Humidity:
💧 %d%%`, ts, v.Description, v.MinimumTemperature, v.MaximumTemperature, v.WindSpeed, v.Humidity))
	}

	sb.WriteString("\n- - - - - - - - - - - - - - - - - - - - - -")

	return sb.String()
}

func telegramWebhookController(locClient location.Client, weatherClient weather.Client) func(c *gin.Context) {
	return func(c *gin.Context) {
		var p *tgram.WebhookRequest

		if i, ok := c.Get(CtxKeyPayload); ok {
			p = i.(*tgram.WebhookRequest)
		} else {
			panic("how did we get here without the payload?")
		}

		if strings.Contains(p.Message.Text, "/today") {
			strs := strings.Split(p.Message.Text, " ")
			query := strings.Join(strs[1:], " ")

			log.Printf("fetching location for %s", query)
			location, err := locClient.FindLocation(query)
			if err != nil {
				log.Printf("error: %s", err.Error())
				c.JSON(200, webhookResponse(p, fmt.Sprintf("aww man, couldn't get your weather report: %s!", err.Error())))
				return
			}

			log.Printf("fetching weather for %s", location.Name)
			forecasts, err := weatherClient.GetUpcomingWeather(location.Latitude, location.Longitude)
			if err != nil {
				log.Printf("error: %s", err.Error())
				c.JSON(200, webhookResponse(p, fmt.Sprintf("aww man, couldn't get your weather report: %s!", err.Error())))
				return
			}

			c.JSON(200, webhookResponse(p, BuildMessage(forecasts)))
			return
		}

		c.JSON(200, webhookResponse(p, fmt.Sprintf("hey %s!", p.Message.Chat.Username)))
	}
}

func newWeatherClient() (weather.Client, error) {
	var openWeatherMapAPIKey string
	if openWeatherMapAPIKey = os.Getenv("OPENWEATHERMAP_API_KEY"); openWeatherMapAPIKey == "" {
		return nil, fmt.Errorf("missing OPENWEATHERMAP_API_KEY environment variable. Please check your environment.")
	}

	return weather.NewOpenWeatherMapClient(&http.Client{Timeout: 5 * time.Second}, openWeatherMapAPIKey), nil
}

func newLocationClient() (location.Client, error) {
	var positionStackAPIKey string
	if positionStackAPIKey = os.Getenv("POSITIONSTACK_API_KEY"); positionStackAPIKey == "" {
		return nil, fmt.Errorf("missing POSITIONSTACK_API_KEY environment variable. Please check your environment.")
	}

	return location.NewPositionStackClient(&http.Client{Timeout: 5 * time.Second}, positionStackAPIKey), nil
}

func newTelegramClient() (tgram.Client, error) {
	var telegramBotToken string
	if telegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN"); telegramBotToken == "" {
		return nil, fmt.Errorf("missing TELEGRAM_BOT_TOKEN environment variable. Please check your environment.")
	}

	return tgram.NewClient(&http.Client{Timeout: 5 * time.Second}, telegramBotToken), nil
}

func newUsersClient() (users.UsersClient, *grpc.ClientConn, error) {
	var host string
	if host = os.Getenv("USERS_SERVICE_HOST"); host == "" {
		return nil, nil, fmt.Errorf("missing TELEGRAM_BOT_TOKEN environment variable. Please check your environment.")
	}

	var opts []grpc.DialOption
	conn, err := grpc.Dial(host, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to connect with users service: %w", err)
	}

	c := users.NewUsersClient(conn)
	return c, conn, nil
}
