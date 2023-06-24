// package env contains simple getters for common abstractions that rely on
// shared environment variables (railway concept).
//
// A shared environment variable is simply a variable that is shared across
// services.
package env

import (
	"fmt"
	"os"
	"strconv"

	"github.com/manzanit0/weathry/pkg/tgram"
	"github.com/manzanit0/weathry/pkg/whttp"
)

// NewErroryTgramClient creates a telegram client which sends messages from the
// @errorybot account. This is simply to report errors.
func NewErroryTgramClient() (tgram.Client, error) {
	var telegramBotToken string
	if telegramBotToken = os.Getenv("ERRORY_BOT_TOKEN"); telegramBotToken == "" {
		return nil, fmt.Errorf("missing ERRORY_BOT_TOKEN environment variable. Please check your environment.")
	}

	httpClient := whttp.NewLoggingClient()
	return tgram.NewClient(httpClient, telegramBotToken), nil
}

// MyTelegramChatID is the chat ID of @Manzanit0, the developer of this bot.
func MyTelegramChatID() (int64, error) {
	var chatID string
	if chatID = os.Getenv("MY_TELEGRAM_CHAT_ID"); chatID == "" {
		return 0, fmt.Errorf("failed get chat ID from MY_TELEGRAM_CHAT_ID OS enviroment variable")
	}

	chatIDint, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse MY_TELEGRAM_CHAT_ID as integer: %s", err.Error())
	}

	return chatIDint, nil
}
