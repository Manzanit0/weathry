package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type User struct {
	TelegramChatID string  `db:"chat_id"`
	Username       *string `db:"username"`
	FirstName      *string `db:"first_name"`
	LastName       *string `db:"last_name"`
	LanguageCode   string  `db:"language_code"`
	IsBot          string  `db:"is_bot"`
}

type UsersRepository struct {
	DB *sql.DB
}

func (r *UsersRepository) Create(ctx context.Context, u User) (User, error) {
	db := sqlx.NewDb(r.DB, "postgres")
	_, err := db.ExecContext(ctx, `INSERT INTO users (chat_id, username) VALUES ($1, $2)`, fmt.Sprint(u.TelegramChatID), u.Username)
	if err != nil {
		return u, err
	}

	return u, nil
}

func (r *UsersRepository) Find(ctx context.Context, chatID string) (*User, error) {
	var u User

	db := sqlx.NewDb(r.DB, "postgres")
	err := db.GetContext(ctx, &u, `SELECT * FROM users WHERE chat_id = $1`, chatID)
	if err != nil {
		return nil, err
	}

	return &u, nil
}
