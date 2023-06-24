package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/manzanit0/weathry/pkg/env"
	"github.com/manzanit0/weathry/pkg/middleware"
	"golang.org/x/exp/slog"
)

func init() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
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

	myTelegramChatID, err := env.MyTelegramChatID()
	if err != nil {
		panic(err)
	}

	errorTgramClient, err := env.NewErroryTgramClient()
	if err != nil {
		panic(err)
	}

	users := UsersRepository{db}

	r := gin.New()
	r.Use(middleware.Recovery(errorTgramClient, myTelegramChatID))
	r.Use(middleware.Logging())

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.PUT("/users/:id", func(c *gin.Context) {
		u := User{}
		err := c.BindJSON(&u)
		if err != nil {
			slog.Error("unable to bind json", "error", err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
			return
		}

		_, err = users.Find(c.Request.Context(), fmt.Sprint(id))
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to find user", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if errors.Is(err, sql.ErrNoRows) {
			u.TelegramChatID = id
			_, err = users.Create(c.Request.Context(), u)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				slog.Error("failed to create user", "error", err.Error())
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			slog.Info("user created", "telegram.chat_id", u.TelegramChatID)
			c.JSON(http.StatusCreated, gin.H{"id": id})
			return
		}

		slog.Info("user found, ignoring request", "telegram.chat_id", u.TelegramChatID)
		c.JSON(http.StatusAccepted, gin.H{"id": id})
	})

	var port string
	if port = os.Getenv("PORT"); port == "" {
		port = "8080"
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{Addr: fmt.Sprintf(":%s", port), Handler: r}
	go func() {
		slog.Info(fmt.Sprintf("serving HTTP on :%s", port))

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server ended abruptly", "error", err.Error())
		} else {
			slog.Info("server ended gracefully")
		}

		stop()
	}()

	// Listen for OS interrupt
	<-ctx.Done()
	stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("server forced to shutdown: %w", err)
	}

	slog.Info("server exited")
}
