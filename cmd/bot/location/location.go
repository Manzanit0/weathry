package location

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"log/slog"

	"github.com/jmoiron/sqlx"
)

type Location struct {
	Latitude    float64
	Longitude   float64
	Name        string
	Country     string
	CountryCode string
}

type dbLocation struct {
	Name string `db:"name"`

	Latitude  *float64 `db:"latitude"`
	Longitude *float64 `db:"longitude"`

	Country     *string `db:"country"`
	CountryCode *string `db:"country_code"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type dbHome struct {
	Name string `db:"name"`

	Latitude  *float64 `db:"latitude"`
	Longitude *float64 `db:"longitude"`

	Country     *string `db:"country"`
	CountryCode *string `db:"country_code"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	UserID string `db:"user_id"`
}

type HomeLocation struct {
	Location
	UserID int
}

type Repository interface {
	CreateLocation(ctx context.Context, name string) (*Location, error)
	UpdateLocation(ctx context.Context, loc *Location) error
	GetLocation(ctx context.Context, name string) (*Location, error)

	GetHome(ctx context.Context, userID int) (*HomeLocation, error)
	SetHome(ctx context.Context, userID int, location *Location) error
	ListHomes(ctx context.Context) ([]*HomeLocation, error)
}

type pgRepo struct {
	db *sqlx.DB
}

var _ Repository = (*pgRepo)(nil)

func NewPgRepository(db *sql.DB) *pgRepo {
	return &pgRepo{db: sqlx.NewDb(db, "postgres")}
}

func (r *pgRepo) SetHome(ctx context.Context, userID int, location *Location) error {
	query := `SELECT COUNT(*) FROM user_locations WHERE user_id = $1 AND location_name = $2 AND is_home = True`
	var count int
	err := r.db.GetContext(ctx, &count, query, fmt.Sprint(userID), location.Name)
	if err != nil {
		return fmt.Errorf("check home: %w", err)
	}

	// If the location is already home, short-circuit.
	if count > 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		err = tx.Rollback()
		if err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("rollback set home transaction", "error", err.Error())
		}
	}()

	query = `
	UPDATE user_locations
	SET is_home = False
	WHERE user_id = $1;`
	_, err = tx.ExecContext(ctx, query, fmt.Sprint(userID))
	if err != nil {
		return fmt.Errorf("update previous user_location: %w", err)
	}

	query = `
	INSERT INTO user_locations (location_name, user_id, is_home)
	VALUES ($1, $2, True)
	ON CONFLICT (location_name, user_id) DO UPDATE SET is_home = True;`
	_, err = tx.ExecContext(ctx, query, location.Name, fmt.Sprint(userID))
	if err != nil {
		return fmt.Errorf("insert new user_location: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func (r *pgRepo) GetHome(ctx context.Context, userID int) (*HomeLocation, error) {
	var u dbLocation

	query := `
	SELECT lo.*
	FROM user_locations ul
	INNER JOIN locations lo ON lo.name = ul.location_name
	WHERE ul.user_id = $1 AND ul.is_home = True;`

	err := r.db.GetContext(ctx, &u, query, fmt.Sprint(userID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("select user_location: %w", err)
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	home := HomeLocation{Location: Location{Name: u.Name}, UserID: userID}
	return &home, nil
}

func (r *pgRepo) CreateLocation(ctx context.Context, name string) (*Location, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO locations (name) VALUES ($1)`, name)
	if err != nil {
		return nil, fmt.Errorf("insert location: %w", err)
	}

	return &Location{Name: name}, nil
}

func (r *pgRepo) UpdateLocation(ctx context.Context, location *Location) error {
	query := `
	UPDATE locations
	SET latitude = $1, longitude = $2, country = $3, country_code = $4
	WHERE name = $5;`

	_, err := r.db.ExecContext(ctx, query, location.Latitude, location.Longitude, location.Country, location.CountryCode, location.Name)
	if err != nil {
		return fmt.Errorf("update location: %w", err)
	}

	return nil
}

func (r *pgRepo) GetLocation(ctx context.Context, name string) (*Location, error) {
	var u dbLocation

	err := r.db.GetContext(ctx, &u, `SELECT * FROM locations WHERE name = $1 LIMIT 1`, name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("select location: %w", err)
	} else if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	return u.Map(), nil
}

func (r *pgRepo) ListHomes(ctx context.Context) ([]*HomeLocation, error) {
	var homes []dbHome

	query := `
	SELECT lo.*, ul.user_id
	FROM user_locations ul
	INNER JOIN locations lo ON lo.name = ul.location_name
	WHERE ul.is_home = True;`

	err := r.db.SelectContext(ctx, &homes, query)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("select user_location: %w", err)
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	homesx := make([]*HomeLocation, len(homes))
	for i := range homes {
		uid, _ := strconv.Atoi(homes[i].UserID)
		homesx[i] = &HomeLocation{
			Location: Location{
				Latitude:    *homes[i].Latitude,
				Longitude:   *homes[i].Longitude,
				Name:        homes[i].Name,
				Country:     *homes[i].Country,
				CountryCode: *homes[i].CountryCode,
			},
			UserID: uid,
		}
	}

	return homesx, nil
}

func (u dbLocation) Map() *Location {
	loc := Location{Name: u.Name}

	if u.Latitude != nil {
		loc.Latitude = *u.Latitude
	}

	if u.Longitude != nil {
		loc.Longitude = *u.Longitude
	}

	if u.Country != nil {
		loc.Country = *u.Country
	}

	if u.CountryCode != nil {
		loc.CountryCode = *u.CountryCode
	}

	return &loc
}
