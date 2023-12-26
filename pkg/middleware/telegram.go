package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/manzanit0/weathry/pkg/tgram"
)

const CtxKeyPayload = "gin.ctx.payload"

// TelegramAuth validates that the user making the requests is authorised
func TelegramAuth(usersClient UsersClient, authorisedUsers ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var r tgram.WebhookRequest
		if err := c.ShouldBindJSON(&r); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Errorf("payload does not conform with telegram contract: %w", err).Error(),
			})
			return
		}

		authorised := len(authorisedUsers) == 0
		for _, username := range authorisedUsers {
			if strings.EqualFold(r.GetFromUsername(), username) {
				authorised = true
				break
			}
		}

		if !authorised {
			slog.InfoContext(c.Request.Context(), "unauthorised user", "username", r.GetFromUsername(), "chat_id", r.GetFromID())
			c.JSON(http.StatusOK, gin.H{
				"method":  "sendMessage",
				"chat_id": r.GetFromID(),
				"text":    "You're not authorised to talk to me, sorry!",
			})
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
			slog.ErrorContext(c.Request.Context(), "unable to track user", "error", err.Error(), "username", username, "chat_id", r.GetFromID())
		} else {
			slog.InfoContext(c.Request.Context(), "user tracked", "username", username, "chat_id", r.GetFromID())
		}

		c.Next()
	}
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
