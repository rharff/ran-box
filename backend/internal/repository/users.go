package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/naratel/naratel-box/backend/internal/logger"
	"github.com/naratel/naratel-box/backend/internal/model"
)

// ErrEmailExists is returned when attempting to create a user with a duplicate email.
var ErrEmailExists = errors.New("email already registered")

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user and returns the created record.
func (r *UserRepository) Create(ctx context.Context, email, hashedPassword string) (*model.User, error) {
	start := time.Now()
	query := "INSERT INTO users (email, password) VALUES ($1, $2) RETURNING ..."

	user := &model.User{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (email, password)
		 VALUES ($1, $2)
		 RETURNING id, email, password, created_at, updated_at`,
		email, hashedPassword,
	).Scan(&user.ID, &user.Email, &user.Password, &user.CreatedAt, &user.UpdatedAt)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			logger.Info(ctx, "Executed query", logger.QueryAttributes{
				Query: query, DurationMs: duration, RowsAffected: 0,
			})
			return nil, ErrEmailExists
		}
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_INSERT_ERR", Details: fmt.Sprintf("UserRepository.Create: %s", err.Error()),
		})
		return nil, fmt.Errorf("UserRepository.Create: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: 1,
	})
	return user, nil
}

// FindByEmail returns a user by email address.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	start := time.Now()
	query := "SELECT id, email, password, created_at, updated_at FROM users WHERE email = $1"

	user := &model.User{}
	err := r.db.QueryRow(ctx, query, email,
	).Scan(&user.ID, &user.Email, &user.Password, &user.CreatedAt, &user.UpdatedAt)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_QUERY_ERR", Details: fmt.Sprintf("UserRepository.FindByEmail: %s", err.Error()),
		})
		return nil, fmt.Errorf("UserRepository.FindByEmail: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: 1,
	})
	return user, nil
}

// FindByID returns a user by ID.
func (r *UserRepository) FindByID(ctx context.Context, id int64) (*model.User, error) {
	start := time.Now()
	query := "SELECT id, email, password, created_at, updated_at FROM users WHERE id = $1"

	user := &model.User{}
	err := r.db.QueryRow(ctx, query, id,
	).Scan(&user.ID, &user.Email, &user.Password, &user.CreatedAt, &user.UpdatedAt)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_QUERY_ERR", Details: fmt.Sprintf("UserRepository.FindByID: %s", err.Error()),
		})
		return nil, fmt.Errorf("UserRepository.FindByID: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: 1,
	})
	return user, nil
}
