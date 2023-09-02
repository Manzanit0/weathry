package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/manzanit0/weathry/pkg/middleware"
)

type usersDBClient struct {
	repo *usersRepository
}

func NewUsersClient(db *sql.DB) middleware.UsersClient {
	return &usersDBClient{repo: &usersRepository{DB: db}}
}

func (c *usersDBClient) CreateUser(ctx context.Context, req middleware.CreateUserPayload) error {
	_, err := c.repo.Find(ctx, fmt.Sprint(req.ID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("find user: %w", err)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil
	}

	u := dbUser{TelegramChatID: req.ID}
	_, err = c.repo.Create(ctx, u)
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

type usersRepository struct {
	DB *sql.DB
}

func (r *usersRepository) Create(ctx context.Context, u dbUser) (dbUser, error) {
	db := sqlx.NewDb(r.DB, "postgres")
	_, err := db.ExecContext(ctx, `INSERT INTO users (chat_id, username) VALUES ($1, $2)`, fmt.Sprint(u.TelegramChatID), u.Username)
	if err != nil {
		return u, err
	}

	return u, nil
}

func (r *usersRepository) Find(ctx context.Context, chatID string) (*dbUser, error) {
	var u dbUser

	db := sqlx.NewDb(r.DB, "postgres")
	err := db.GetContext(ctx, &u, `SELECT * FROM users WHERE chat_id = $1`, chatID)
	if err != nil {
		return nil, err
	}

	return &u, nil
}
