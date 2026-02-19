package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/naratel/naratel-box/backend/internal/model"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user and returns the created record.
func (r *UserRepository) Create(ctx context.Context, email, hashedPassword string) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (email, password)
		 VALUES ($1, $2)
		 RETURNING id, email, password, created_at, updated_at`,
		email, hashedPassword,
	).Scan(&user.ID, &user.Email, &user.Password, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("UserRepository.Create: %w", err)
	}
	return user, nil
}

// FindByEmail returns a user by email address.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, email, password, created_at, updated_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.Password, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("UserRepository.FindByEmail: %w", err)
	}
	return user, nil
}

// FindByID returns a user by ID.
func (r *UserRepository) FindByID(ctx context.Context, id int64) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(ctx,
		`SELECT id, email, password, created_at, updated_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.Password, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("UserRepository.FindByID: %w", err)
	}
	return user, nil
}
