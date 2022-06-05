package main

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

type User struct {
	TelegramChatID int64  `db:"chat_id"`
	Username       string `db:"username"`
	FirstName      string `db:"first_name"`
	LastName       string `db:"last_name"`
	LanguageCode   string `db:"language_code"`
}

type UsersRepository struct {
	DB *sql.DB
}

func (r *UsersRepository) Create(ctx context.Context, u User) (User, error) {
	db := sqlx.NewDb(r.DB, "postgres")
	_, err := db.ExecContext(ctx, `INSERT INTO users (chat_id, username) VALUES (?, ?)`, u.TelegramChatID, u.Username)
	if err != nil {
		return u, err
	}

	return u, nil
}

func (r *UsersRepository) Find(ctx context.Context, chatID string) (*User, error) {
	var u User

	db := sqlx.NewDb(r.DB, "postgres")
	err := db.SelectContext(ctx, &u, `SELECT * FROM users WHERE chat_id = ?`, chatID)
	if err != nil {
		return nil, err
	}

	return &u, nil
}
