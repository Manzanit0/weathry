package conversation

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

const (
	QuestionHourlyWeather = "AWAITING_HOURLY_WEATHER_CITY"
	QuestionDailyWeather  = "AWAITING_DAILY_WEATHER_CITY"
	QuestionHome          = "AWAITING_HOME"
)

type ConversationState struct {
	TelegramChatID    string `db:"chat_id"`
	LastQuestionAsked string `db:"last_question_asked"`
	Answered          bool   `db:"answered"`
}

type ConvoRepository struct {
	DB *sql.DB
}

func (r *ConvoRepository) AddQuestion(ctx context.Context, chatID, question string) (ConversationState, error) {
	query := `
		INSERT INTO conversation_states (chat_id, last_question_asked)
		VALUES ($1, $2)
		ON CONFLICT (chat_id)
		DO UPDATE SET last_question_asked = $2, answered = false;`
	db := sqlx.NewDb(r.DB, "postgres")
	_, err := db.ExecContext(ctx, query, chatID, question)
	if err != nil {
		return ConversationState{}, err
	}

	return ConversationState{TelegramChatID: chatID, LastQuestionAsked: question, Answered: false}, nil
}

func (r *ConvoRepository) MarkQuestionAnswered(ctx context.Context, chatID string) error {
	query := `
		UPDATE conversation_states
		SET answered = true
		WHERE chat_id = $1 AND answered = false`
	db := sqlx.NewDb(r.DB, "postgres")
	_, err := db.ExecContext(ctx, query, chatID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	return nil
}

func (r *ConvoRepository) Find(ctx context.Context, chatID string) (*ConversationState, error) {
	var s ConversationState

	db := sqlx.NewDb(r.DB, "postgres")
	err := db.GetContext(ctx, &s, `SELECT chat_id, last_question_asked, answered FROM conversation_states WHERE chat_id = $1`, chatID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	return &s, nil
}
