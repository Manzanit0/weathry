package main

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
)

type User struct {
	TelegramChatID int64
	Username       string
	FirstName      string
	LastName       string
	LanguageCode   string
}

type UsersRepository struct {
	DB *sql.DB
}

func (r *UsersRepository) Create(u User) (User, error) {
	db := sqlx.NewDb(r.DB, "postgres")
	res := db.MustExec(`INSERT INTO users (chat_id, username) VALUES (?, ?)`, u.TelegramChatID, u.Username)
	_, err := res.RowsAffected()
	if err != nil {
		return u, err
	}

	return u, nil
}
