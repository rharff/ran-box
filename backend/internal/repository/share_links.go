package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/naratel/naratel-box/backend/internal/model"
)

type ShareLinkRepository struct {
	db *pgxpool.Pool
}

func NewShareLinkRepository(db *pgxpool.Pool) *ShareLinkRepository {
	return &ShareLinkRepository{db: db}
}

// Create inserts a new share link.
func (r *ShareLinkRepository) Create(ctx context.Context, fileID, userID int64, token string, expiresAt *time.Time) (*model.ShareLink, error) {
	link := &model.ShareLink{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO share_links (file_id, user_id, token, expires_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, file_id, user_id, token, expires_at, created_at`,
		fileID, userID, token, expiresAt,
	).Scan(&link.ID, &link.FileID, &link.UserID, &link.Token, &link.ExpiresAt, &link.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("ShareLinkRepository.Create: %w", err)
	}
	return link, nil
}

// FindByToken returns a share link by its unique token.
func (r *ShareLinkRepository) FindByToken(ctx context.Context, token string) (*model.ShareLink, error) {
	link := &model.ShareLink{}
	err := r.db.QueryRow(ctx,
		`SELECT id, file_id, user_id, token, expires_at, created_at
		 FROM share_links WHERE token = $1`,
		token,
	).Scan(&link.ID, &link.FileID, &link.UserID, &link.Token, &link.ExpiresAt, &link.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("ShareLinkRepository.FindByToken: %w", err)
	}
	return link, nil
}

// FindByFileID returns share links for a file.
func (r *ShareLinkRepository) FindByFileID(ctx context.Context, fileID, userID int64) ([]*model.ShareLink, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, file_id, user_id, token, expires_at, created_at
		 FROM share_links WHERE file_id = $1 AND user_id = $2
		 ORDER BY created_at DESC`,
		fileID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("ShareLinkRepository.FindByFileID: %w", err)
	}
	defer rows.Close()

	var links []*model.ShareLink
	for rows.Next() {
		l := &model.ShareLink{}
		if err := rows.Scan(&l.ID, &l.FileID, &l.UserID, &l.Token, &l.ExpiresAt, &l.CreatedAt); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, nil
}

// Delete removes a share link.
func (r *ShareLinkRepository) Delete(ctx context.Context, linkID, userID int64) error {
	result, err := r.db.Exec(ctx,
		`DELETE FROM share_links WHERE id = $1 AND user_id = $2`,
		linkID, userID,
	)
	if err != nil {
		return fmt.Errorf("ShareLinkRepository.Delete: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("share link not found or unauthorized")
	}
	return nil
}
