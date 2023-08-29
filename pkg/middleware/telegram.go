package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/manzanit0/weathry/pkg/tgram"
	"github.com/manzanit0/weathry/pkg/whttp"
	"golang.org/x/exp/slog"
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

		var authorised bool
		for _, username := range authorisedUsers {
			if strings.EqualFold(r.GetFromUsername(), username) {
				authorised = true
				break
			}
		}

		if !authorised {
			slog.Info("unauthorised user", "username", r.GetFromUsername())
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
			slog.Error("unable to track user", "error", err.Error(), "username", username)
		} else {
			slog.Info("user tracked", "username", username)
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

type usersClient struct {
	host string
	h    *http.Client
}

func NewUsersClient(host string) UsersClient {
	h := whttp.NewLoggingClient()
	h.Timeout = 1 * time.Second // This should be taking millis...
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
		return err
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
