package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
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
	"github.com/manzanit0/weathry/pkg/middleware"
	"github.com/manzanit0/weathry/pkg/pings"
	"github.com/manzanit0/weathry/pkg/tgram"
	"github.com/manzanit0/weathry/pkg/weather"
	"github.com/manzanit0/weathry/pkg/whttp"
	"github.com/olekukonko/tablewriter"

	_ "github.com/jackc/pgx/v4/stdlib"
)

const CtxKeyPayload = "gin.ctx.payload"

func main() {
	db, err := sql.Open("pgx", fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", os.Getenv("PGUSER"), os.Getenv("PGPASSWORD"), os.Getenv("PGHOST"), os.Getenv("PGPORT"), os.Getenv("PGDATABASE")))
	if err != nil {
		panic(fmt.Errorf("unable to open db conn: %w", err))
	}

	defer func() {
		err = db.Close()
		if err != nil {
			log.Printf("error closing db connection: %s\n", err.Error())
		}
	}()

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

	usersClient, err := newUsersClient()
	if err != nil {
		panic(err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logging(log.Default()))

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
		log.Printf("serving HTTP on :%s", port)

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
			log.Printf("unauthorised user: %s", r.GetFromUsername())
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
			log.Printf("unable to track user: %s\n", err.Error())
		} else {
			log.Printf("user tracked: %s\n", username)
		}

		c.Next()
	}
}

func BuildMessage(f []*weather.Forecast) string {
	if len(f) == 0 {
		return "hey, not sure why but I couldn't get any forecasts ¯\\_(ツ)_/¯"
	}

	// TODO: extract from here...
	// we just want the next 3 forecasts
	ff := make([]*weather.Forecast, 3)
	for i := 0; i < 3; i++ {
		ff[i] = f[i]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Weather Report for %s", f[0].Location))
	for _, v := range f {
		ts := v.FormattedDateTime()
		sb.WriteString(fmt.Sprintf(`
- - - - - - - - - - - - - - - - - - - - - -
📅 %s
🏷 %s
🌡 %0.2f°C - %0.2f°C
💨 %0.2f m/s
💧 %d%%`, ts, v.Description, v.MinimumTemperature, v.MaximumTemperature, v.WindSpeed, v.Humidity))
	}

	sb.WriteString("\n- - - - - - - - - - - - - - - - - - - - - -")

	return sb.String()
}

func BuildHourlyMessage(f []*weather.Forecast) string {
	if len(f) == 0 {
		return "hey, not sure why but I couldn't get any forecasts ¯\\_(ツ)_/¯"
	}

	// TODO: extract from here...
	// we just want the next 9 forecasts
	ff := make([]*weather.Forecast, 9)
	for i := 0; i < 9; i++ {
		ff[i] = f[i]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Weather Report for %s", f[0].Location))
	for _, v := range ff {
		ts, err := v.LocalTime("Europe/Madrid")
		if err != nil {
			ts = v.FormattedDateTime()
		}

		sb.WriteString(fmt.Sprintf(`
- - - - - - - - - - - - - - - - - - - - - -
📅 %s
🏷 %s
🌡 %0.2f°C
💨 %0.2f m/s
💧 %d%%`, ts, v.Description, v.MinimumTemperature, v.WindSpeed, v.Humidity))
	}

	sb.WriteString("\n- - - - - - - - - - - - - - - - - - - - - -")

	return sb.String()
}

type messageOptions struct {
	withTempDiff bool
	withTime     bool
}

type MessageOption func(*messageOptions)

func WithTemperatureDiff() MessageOption {
	return func(config *messageOptions) {
		config.withTempDiff = true
	}
}

func WithTime() MessageOption {
	return func(config *messageOptions) {
		config.withTime = true
	}
}

func BuildMessageAsTable(loc *location.Location, f []*weather.Forecast, opts ...MessageOption) string {
	if len(f) == 0 {
		return "hey, not sure why but I couldn't get any forecasts ¯\\_(ツ)_/¯"
	}

	options := messageOptions{withTempDiff: false, withTime: false}
	for _, f := range opts {
		f(&options)
	}

	b := bytes.NewBuffer([]byte{})
	table := tablewriter.NewWriter(b)

	if options.withTime {
		table.SetHeader([]string{"Time", "Report"})
	} else {

		table.SetHeader([]string{"Date", "Report"})
	}

	for _, v := range f {
		temp := fmt.Sprintf("%.0fºC", v.MinimumTemperature)
		if options.withTempDiff {
			temp = fmt.Sprintf("%.0fºC - %.0fºC", v.MinimumTemperature, v.MaximumTemperature)
		}

		dt := v.FormattedDate()
		if options.withTime {
			dt = v.FormattedTime()
		}

		table.Append([]string{dt, fmt.Sprintf("%s  \n%s", v.Description, temp)})
	}

	table.SetRowLine(true)
	table.SetRowSeparator("-")
	table.SetAutoFormatHeaders(false)

	table.Render()

	// we're making the assumption here that all forecasts belong to the same day.
	if options.withTime {
		return fmt.Sprintf("```%s  \n%s  \n%s```",
			time.Unix(int64(f[0].DateTimeTS), 0).Format("Mon, 02 Jan 2006"),
			loc.Name,
			b.String(),
		)

	}

	return fmt.Sprintf("```%s  \n%s```",
		loc.Name,
		b.String(),
	)
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
			c.JSON(200, webhookResponse(p, "Unsupported type of interaction"))
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

				c.JSON(200, webhookResponse(p, "What location do you want me to check this week's weather for?"))
				return
			}

			if convo, err := convos.Find(c.Request.Context(), fmt.Sprint(p.GetFromID())); err == nil && convo != nil && !convo.Answered {
				err = convos.MarkQuestionAnswered(c.Request.Context(), fmt.Sprint(p.GetFromID()))
				if err != nil {
					log.Printf("error: unable to mark question as answered: %s", err.Error())
				}
			}

			message, err := GetDailyWeather(locClient, weatherClient, query)
			if err != nil {
				log.Printf("error: get upcoming weather: %s", err.Error())
				c.JSON(200, webhookResponse(p, fmt.Sprintf("aww man, couldn't get your weather report: %s!", err.Error())))
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

				c.JSON(200, webhookResponse(p, "What location do you want me to check today's weather for?"))
				return
			}

			if convo, err := convos.Find(c.Request.Context(), fmt.Sprint(p.GetFromID())); err == nil && convo != nil && !convo.Answered {
				err = convos.MarkQuestionAnswered(c.Request.Context(), fmt.Sprint(p.GetFromID()))
				if err != nil {
					log.Printf("error: unable to mark question as answered: %s", err.Error())
				}
			}

			message, err := GetHourlyWeather(locClient, weatherClient, query)
			if err != nil {
				log.Printf("error: get upcoming weather: %s", err.Error())
				c.JSON(200, webhookResponse(p, fmt.Sprintf("aww man, couldn't get your weather report: %s!", err.Error())))
				return
			}

			c.JSON(200, webhookResponse(p, message))
		default:
			log.Println("received non-command query")
			convo, err := convos.Find(c.Request.Context(), fmt.Sprint(p.GetFromID()))
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				panic(err)
			} else if errors.Is(err, sql.ErrNoRows) || (convo != nil && convo.Answered) {
				c.JSON(200, webhookResponse(p, "I\\'m not sure what you mean with that\\. Try hitting me up with the /hourly or /daily commands if you need me to check the weather for you \\:\\)"))
				return
			}

			response := getQuery(p.Message.Text)
			message, err := forecastFromQuestion(locClient, weatherClient, convo.LastQuestionAsked, response)
			if err != nil {
				log.Printf("error: get forecast from question: %s", err.Error())
				c.JSON(200, webhookResponse(p, fmt.Sprintf("aww man, couldn't get your weather report: %s!", err.Error())))
				return
			}

			err = convos.MarkQuestionAnswered(c.Request.Context(), fmt.Sprint(p.GetFromID()))
			if err != nil {
				log.Printf("error: unable to mark question as answered: %s", err.Error())
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

	return BuildMessageAsTable(location, forecasts, WithTemperatureDiff()), nil
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
	ff := make([]*weather.Forecast, 9)
	for i := 0; i < 9; i++ {
		ff[i] = forecasts[i]
	}

	return BuildMessageAsTable(location, ff, WithTime()), nil
}

func getQuery(text string) string {
	strs := strings.Split(text, " ")
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

	log.Printf("status code from calling users: %d", resp.StatusCode)
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
