package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/naratel/naratel-box/backend/internal/logger"
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
	start := time.Now()
	query := "INSERT INTO share_links (file_id, user_id, token, expires_at) VALUES ($1, $2, $3, $4) RETURNING ..."

	link := &model.ShareLink{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO share_links (file_id, user_id, token, expires_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, file_id, user_id, token, expires_at, created_at`,
		fileID, userID, token, expiresAt,
	).Scan(&link.ID, &link.FileID, &link.UserID, &link.Token, &link.ExpiresAt, &link.CreatedAt)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_INSERT_ERR", Details: fmt.Sprintf("ShareLinkRepository.Create: %s", err.Error()),
		})
		return nil, fmt.Errorf("ShareLinkRepository.Create: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: 1,
	})
	return link, nil
}

// FindByToken returns a share link by its unique token.
func (r *ShareLinkRepository) FindByToken(ctx context.Context, token string) (*model.ShareLink, error) {
	start := time.Now()
	query := "SELECT id, file_id, user_id, token, expires_at, created_at FROM share_links WHERE token = $1"

	link := &model.ShareLink{}
	err := r.db.QueryRow(ctx, query, token,
	).Scan(&link.ID, &link.FileID, &link.UserID, &link.Token, &link.ExpiresAt, &link.CreatedAt)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Info(ctx, "Executed query", logger.QueryAttributes{
				Query: query, DurationMs: duration, RowsAffected: 0,
			})
			return nil, nil
		}
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_QUERY_ERR", Details: fmt.Sprintf("ShareLinkRepository.FindByToken: %s", err.Error()),
		})
		return nil, fmt.Errorf("ShareLinkRepository.FindByToken: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: 1,
	})
	return link, nil
}

// FindByFileID returns share links for a file.
func (r *ShareLinkRepository) FindByFileID(ctx context.Context, fileID, userID int64) ([]*model.ShareLink, error) {
	start := time.Now()
	query := "SELECT id, file_id, user_id, token, expires_at, created_at FROM share_links WHERE file_id = $1 AND user_id = $2 ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, query, fileID, userID)
	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_QUERY_ERR", Details: fmt.Sprintf("ShareLinkRepository.FindByFileID: %s", err.Error()),
		})
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

	duration := time.Since(start).Milliseconds()
	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: int64(len(links)),
	})
	return links, nil
}

// Delete removes a share link.
func (r *ShareLinkRepository) Delete(ctx context.Context, linkID, userID int64) error {
	start := time.Now()
	query := "DELETE FROM share_links WHERE id = $1 AND user_id = $2"

	result, err := r.db.Exec(ctx, query, linkID, userID)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_DELETE_ERR", Details: fmt.Sprintf("ShareLinkRepository.Delete: %s", err.Error()),
		})
		return fmt.Errorf("ShareLinkRepository.Delete: %w", err)
	}
	if result.RowsAffected() == 0 {
		logger.Warn(ctx, "Delete affected 0 rows", map[string]interface{}{
			"link_id": linkID, "user_id": userID,
		})
		return fmt.Errorf("share link not found or unauthorized")
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: result.RowsAffected(),
	})
	return nil
}
