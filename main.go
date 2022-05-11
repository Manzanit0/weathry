package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/manzanit0/weathry/pkg/location"
	"github.com/manzanit0/weathry/pkg/weather"
)

const CtxKeyPayload = "gin.ctx.payload"

func main() {
	var openWeatherMapAPIKey string
	if openWeatherMapAPIKey = os.Getenv("OPENWEATHERMAP_API_KEY"); openWeatherMapAPIKey == "" {
		panic("missing OPENWEATHERMAP_API_KEY environment variable. Please check your environment.")
	}

	var positionStackAPIKey string
	if positionStackAPIKey = os.Getenv("POSITIONSTACK_API_KEY"); positionStackAPIKey == "" {
		panic("missing POSITIONSTACK_API_KEY environment variable. Please check your environment.")
	}

	owmClient := weather.NewOpenWeatherMapClient(&http.Client{Timeout: 5 * time.Second}, openWeatherMapAPIKey)
	psClient := location.NewPositionStackClient(&http.Client{Timeout: 5 * time.Second}, positionStackAPIKey)

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.Use(TelegramAuth())
	r.POST("/telegram/webhook", func(c *gin.Context) {
		var p *WebhookRequest

		if i, ok := c.Get(CtxKeyPayload); ok {
			p = i.(*WebhookRequest)
		} else {
			panic("how did we get here without the payload?")
		}

		if strings.Contains(p.Message.Text, "/today") {
			strs := strings.Split(p.Message.Text, " ")
			query := strings.Join(strs[1:], " ")

			log.Printf("fetching location for %s\n", query)
			location, err := psClient.FindLocation(query)
			if err != nil {
				log.Printf("error: %s\n", err.Error())
				c.JSON(200, webhookResponse(p, fmt.Sprintf("aww man, couldn't get your weather report: %s!", err.Error())))
				return
			}

			log.Printf("fetching weather for %s\n", location.Name)
			forecast, err := owmClient.GetCurrentWeatherByCoordinates(location.Latitude, location.Longitude)
			if err != nil {
				log.Printf("error: %s\n", err.Error())
				c.JSON(200, webhookResponse(p, fmt.Sprintf("aww man, couldn't get your weather report: %s!", err.Error())))
				return
			}

			c.JSON(200, webhookResponse(p, fmt.Sprintf("it's %s in %s", forecast.Description, location.Name)))
			return
		}

		c.JSON(200, webhookResponse(p, fmt.Sprintf("hey %s!", p.Message.Chat.Username)))
	})

	var port string
	if port = os.Getenv("PORT"); port == "" {
		port = "8080"
	}

	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		panic(err)
	}
}

func webhookResponse(p *WebhookRequest, text string) gin.H {
	return gin.H{
		"method":  "sendMessage",
		"chat_id": p.Message.From.ID,
		"text":    text,
	}
}

func TelegramAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var r WebhookRequest
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
		c.Next()
	}
}

type WebhookRequest struct {
	UpdateID int     `json:"update_id"`
	Message  Message `json:"message"`
}

type From struct {
	ID           int    `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
}

type Chat struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Type      string `json:"type"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	From      From   `json:"from"`
	Chat      Chat   `json:"chat"`
	Date      int    `json:"date"`
	Text      string `json:"text"`
}
