package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/manzanit0/weathry/pkg/middleware"
)

type repository struct {
	dbx *sqlx.DB
}

func NewDBClient(db *sql.DB) middleware.UsersClient {
	dbx := sqlx.NewDb(db, "postgres")
	return &repository{dbx: dbx}
}

func (c *repository) CreateUser(ctx context.Context, req middleware.CreateUserPayload) error {
	var u dbUser
	err := c.dbx.GetContext(ctx, &u, `SELECT * FROM users WHERE chat_id = $1`, fmt.Sprint(req.ID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("find user: %w", err)
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}

	u = dbUser{TelegramChatID: req.ID}
	_, err = c.dbx.ExecContext(ctx, `INSERT INTO users (chat_id, username) VALUES ($1, $2)`, fmt.Sprint(u.TelegramChatID), u.Username)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

type dbUser struct {
	TelegramChatID string  `db:"chat_id"`
	Username       *string `db:"username"`
	FirstName      *string `db:"first_name"`
	LastName       *string `db:"last_name"`
	LanguageCode   string  `db:"language_code"`
	IsBot          string  `db:"is_bot"`
}
