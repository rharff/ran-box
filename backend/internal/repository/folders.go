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

type FolderRepository struct {
	db *pgxpool.Pool
}

func NewFolderRepository(db *pgxpool.Pool) *FolderRepository {
	return &FolderRepository{db: db}
}

// Create inserts a new folder.
func (r *FolderRepository) Create(ctx context.Context, userID int64, parentID *int64, name string) (*model.Folder, error) {
	start := time.Now()
	query := "INSERT INTO folders (user_id, parent_id, name) VALUES ($1, $2, $3) RETURNING ..."

	folder := &model.Folder{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO folders (user_id, parent_id, name)
		 VALUES ($1, $2, $3)
		 RETURNING id, user_id, parent_id, name, created_at, updated_at`,
		userID, parentID, name,
	).Scan(&folder.ID, &folder.UserID, &folder.ParentID, &folder.Name, &folder.CreatedAt, &folder.UpdatedAt)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_INSERT_ERR", Details: fmt.Sprintf("FolderRepository.Create: %s", err.Error()),
		})
		return nil, fmt.Errorf("FolderRepository.Create: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: 1,
	})
	return folder, nil
}

// FindByIDAndUserID fetches a folder by ID and user ownership.
func (r *FolderRepository) FindByIDAndUserID(ctx context.Context, folderID, userID int64) (*model.Folder, error) {
	start := time.Now()
	query := "SELECT id, user_id, parent_id, name, created_at, updated_at FROM folders WHERE id = $1 AND user_id = $2"

	folder := &model.Folder{}
	err := r.db.QueryRow(ctx, query, folderID, userID,
	).Scan(&folder.ID, &folder.UserID, &folder.ParentID, &folder.Name, &folder.CreatedAt, &folder.UpdatedAt)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Info(ctx, "Executed query", logger.QueryAttributes{
				Query: query, DurationMs: duration, RowsAffected: 0,
			})
			return nil, nil
		}
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_QUERY_ERR", Details: fmt.Sprintf("FolderRepository.FindByIDAndUserID: %s", err.Error()),
		})
		return nil, fmt.Errorf("FolderRepository.FindByIDAndUserID: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: 1,
	})
	return folder, nil
}

// ListByParent returns subfolders within a parent folder (nil = root).
func (r *FolderRepository) ListByParent(ctx context.Context, userID int64, parentID *int64) ([]*model.Folder, error) {
	start := time.Now()
	var query string
	var rows interface {
		Next() bool
		Scan(dest ...interface{}) error
		Close()
	}

	if parentID == nil {
		query = "SELECT id, user_id, parent_id, name, created_at, updated_at FROM folders WHERE user_id = $1 AND parent_id IS NULL ORDER BY name ASC"
		r2, err := r.db.Query(ctx, query, userID)
		if err != nil {
			logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
				Code: "DB_QUERY_ERR", Details: fmt.Sprintf("FolderRepository.ListByParent: %s", err.Error()),
			})
			return nil, fmt.Errorf("FolderRepository.ListByParent: %w", err)
		}
		rows = r2
		defer r2.Close()
	} else {
		query = "SELECT id, user_id, parent_id, name, created_at, updated_at FROM folders WHERE user_id = $1 AND parent_id = $2 ORDER BY name ASC"
		r2, err := r.db.Query(ctx, query, userID, *parentID)
		if err != nil {
			logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
				Code: "DB_QUERY_ERR", Details: fmt.Sprintf("FolderRepository.ListByParent: %s", err.Error()),
			})
			return nil, fmt.Errorf("FolderRepository.ListByParent: %w", err)
		}
		rows = r2
		defer r2.Close()
	}

	var folders []*model.Folder
	for rows.Next() {
		f := &model.Folder{}
		if err := rows.Scan(&f.ID, &f.UserID, &f.ParentID, &f.Name, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}

	duration := time.Since(start).Milliseconds()
	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: int64(len(folders)),
	})
	return folders, nil
}

// Rename updates the name of a folder.
func (r *FolderRepository) Rename(ctx context.Context, folderID, userID int64, newName string) (*model.Folder, error) {
	start := time.Now()
	query := "UPDATE folders SET name = $1, updated_at = NOW() WHERE id = $2 AND user_id = $3 RETURNING ..."

	folder := &model.Folder{}
	err := r.db.QueryRow(ctx,
		`UPDATE folders SET name = $1, updated_at = NOW()
		 WHERE id = $2 AND user_id = $3
		 RETURNING id, user_id, parent_id, name, created_at, updated_at`,
		newName, folderID, userID,
	).Scan(&folder.ID, &folder.UserID, &folder.ParentID, &folder.Name, &folder.CreatedAt, &folder.UpdatedAt)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_UPDATE_ERR", Details: fmt.Sprintf("FolderRepository.Rename: %s", err.Error()),
		})
		return nil, fmt.Errorf("FolderRepository.Rename: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: 1,
	})
	return folder, nil
}

// Move moves a folder to a new parent.
func (r *FolderRepository) Move(ctx context.Context, folderID, userID int64, newParentID *int64) (*model.Folder, error) {
	start := time.Now()
	query := "UPDATE folders SET parent_id = $1, updated_at = NOW() WHERE id = $2 AND user_id = $3 RETURNING ..."

	folder := &model.Folder{}
	err := r.db.QueryRow(ctx,
		`UPDATE folders SET parent_id = $1, updated_at = NOW()
		 WHERE id = $2 AND user_id = $3
		 RETURNING id, user_id, parent_id, name, created_at, updated_at`,
		newParentID, folderID, userID,
	).Scan(&folder.ID, &folder.UserID, &folder.ParentID, &folder.Name, &folder.CreatedAt, &folder.UpdatedAt)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_UPDATE_ERR", Details: fmt.Sprintf("FolderRepository.Move: %s", err.Error()),
		})
		return nil, fmt.Errorf("FolderRepository.Move: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: 1,
	})
	return folder, nil
}

// Delete removes a folder and all its contents (cascades via FK).
func (r *FolderRepository) Delete(ctx context.Context, folderID, userID int64) error {
	start := time.Now()
	query := "DELETE FROM folders WHERE id = $1 AND user_id = $2"

	result, err := r.db.Exec(ctx, query, folderID, userID)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_DELETE_ERR", Details: fmt.Sprintf("FolderRepository.Delete: %s", err.Error()),
		})
		return fmt.Errorf("FolderRepository.Delete: %w", err)
	}
	if result.RowsAffected() == 0 {
		logger.Warn(ctx, "Delete affected 0 rows", map[string]interface{}{
			"folder_id": folderID, "user_id": userID,
		})
		return fmt.Errorf("folder not found or unauthorized")
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: result.RowsAffected(),
	})
	return nil
}

// GetBreadcrumb returns the ancestry chain from root to the given folder.
func (r *FolderRepository) GetBreadcrumb(ctx context.Context, folderID, userID int64) ([]*model.Folder, error) {
	start := time.Now()
	query := "WITH RECURSIVE ancestors AS (...) SELECT id, user_id, parent_id, name, created_at, updated_at FROM ancestors"

	rows, err := r.db.Query(ctx,
		`WITH RECURSIVE ancestors AS (
			SELECT id, user_id, parent_id, name, created_at, updated_at
			FROM folders WHERE id = $1 AND user_id = $2
			UNION ALL
			SELECT f.id, f.user_id, f.parent_id, f.name, f.created_at, f.updated_at
			FROM folders f INNER JOIN ancestors a ON f.id = a.parent_id
		)
		SELECT id, user_id, parent_id, name, created_at, updated_at FROM ancestors`,
		folderID, userID,
	)
	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_QUERY_ERR", Details: fmt.Sprintf("FolderRepository.GetBreadcrumb: %s", err.Error()),
		})
		return nil, fmt.Errorf("FolderRepository.GetBreadcrumb: %w", err)
	}
	defer rows.Close()

	var chain []*model.Folder
	for rows.Next() {
		f := &model.Folder{}
		if err := rows.Scan(&f.ID, &f.UserID, &f.ParentID, &f.Name, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		chain = append(chain, f)
	}

	duration := time.Since(start).Milliseconds()
	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: int64(len(chain)),
	})

	// Reverse so root comes first
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// ListAllByUser returns all folders for a user (for move dialog).
func (r *FolderRepository) ListAllByUser(ctx context.Context, userID int64) ([]*model.Folder, error) {
	start := time.Now()
	query := "SELECT id, user_id, parent_id, name, created_at, updated_at FROM folders WHERE user_id = $1 ORDER BY name ASC"

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_QUERY_ERR", Details: fmt.Sprintf("FolderRepository.ListAllByUser: %s", err.Error()),
		})
		return nil, fmt.Errorf("FolderRepository.ListAllByUser: %w", err)
	}
	defer rows.Close()

	var folders []*model.Folder
	for rows.Next() {
		f := &model.Folder{}
		if err := rows.Scan(&f.ID, &f.UserID, &f.ParentID, &f.Name, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}

	duration := time.Since(start).Milliseconds()
	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: int64(len(folders)),
	})
	return folders, nil
}
