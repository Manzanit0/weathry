package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/manzanit0/weathry/cmd/bot/location"
	"github.com/manzanit0/weathry/cmd/pinger/pings"
	"github.com/manzanit0/weathry/pkg/env"
	"github.com/manzanit0/weathry/pkg/geocode"
	"github.com/manzanit0/weathry/pkg/middleware"
	"github.com/manzanit0/weathry/pkg/tgram"
	"github.com/manzanit0/weathry/pkg/weather"
	"github.com/manzanit0/weathry/pkg/whttp"
	"golang.org/x/exp/slog"

	_ "github.com/jackc/pgx/v4/stdlib"
)

func init() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger = logger.With("service", "pinger")
	slog.SetDefault(logger)
}

func main() {
	slog.Info("starting pinger")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("pinger shutdown abruptly", "error", err.Error())
		os.Exit(1)
	}

	slog.Info("pinger shutdown gracefully")
}

func run(ctx context.Context) error {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("open db connection: %w", err)
	}

	defer func() {
		err = db.Close()
		if err != nil {
			slog.Error("close db connection", "error", err.Error())
		}
	}()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	slog.Info("connected to the database successfully")

	locations := location.NewPgRepository(db)

	owmClient, err := newWeatherClient()
	if err != nil {
		return fmt.Errorf("create weather client: %w", err)
	}

	geocoder, err := newGeocoder()
	if err != nil {
		return fmt.Errorf("create geocoder: %w", err)
	}

	tgramClient, err := newTelegramClient()
	if err != nil {
		return fmt.Errorf("create telegram client: %w", err)
	}

	errorTgramClient, err := env.NewErroryTgramClient()
	if err != nil {
		return fmt.Errorf("create errory telegram client: %w", err)
	}

	myTelegramChatID, err := env.MyTelegramChatID()
	if err != nil {
		return fmt.Errorf("get my telegram chat id: %w", err)
	}

	defer middleware.Recover(errorTgramClient, myTelegramChatID)

	pinger := pings.NewBackgroundPinger(owmClient, geocoder, tgramClient, locations)
	if err := pinger.MonitorWeather(ctx); err != nil {
		return fmt.Errorf("monitor weather: %w", err)
	}

	return nil
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

func newTelegramClient() (tgram.Client, error) {
	var telegramBotToken string
	if telegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN"); telegramBotToken == "" {
		return nil, fmt.Errorf("missing TELEGRAM_BOT_TOKEN environment variable. Please check your environment.")
	}

	httpClient := whttp.NewLoggingClient()
	return tgram.NewClient(httpClient, telegramBotToken), nil
}
