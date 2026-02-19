package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/naratel/naratel-box/backend/internal/model"
)

type FileRepository struct {
	db *pgxpool.Pool
}

func NewFileRepository(db *pgxpool.Pool) *FileRepository {
	return &FileRepository{db: db}
}

// Create inserts a new file record and returns it.
func (r *FileRepository) Create(ctx context.Context, userID int64, name, mimeType string, totalSize int64) (*model.File, error) {
	file := &model.File{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO files (user_id, name, mime_type, total_size)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, name, mime_type, total_size, created_at, updated_at`,
		userID, name, mimeType, totalSize,
	).Scan(&file.ID, &file.UserID, &file.Name, &file.MimeType, &file.TotalSize, &file.CreatedAt, &file.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("FileRepository.Create: %w", err)
	}
	return file, nil
}

// FindByIDAndUserID fetches a file only if it belongs to the given user (ownership check).
func (r *FileRepository) FindByIDAndUserID(ctx context.Context, fileID, userID int64) (*model.File, error) {
	file := &model.File{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, name, mime_type, total_size, created_at, updated_at
		 FROM files WHERE id = $1 AND user_id = $2`,
		fileID, userID,
	).Scan(&file.ID, &file.UserID, &file.Name, &file.MimeType, &file.TotalSize, &file.CreatedAt, &file.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("FileRepository.FindByIDAndUserID: %w", err)
	}
	return file, nil
}

// ListByUserID returns all files for a user ordered by newest first.
func (r *FileRepository) ListByUserID(ctx context.Context, userID int64) ([]*model.File, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, name, mime_type, total_size, created_at, updated_at
		 FROM files WHERE user_id = $1
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("FileRepository.ListByUserID: %w", err)
	}
	defer rows.Close()

	var files []*model.File
	for rows.Next() {
		f := &model.File{}
		if err := rows.Scan(&f.ID, &f.UserID, &f.Name, &f.MimeType, &f.TotalSize, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// Delete removes a file record. Call only after decrementing block ref_counts.
func (r *FileRepository) Delete(ctx context.Context, fileID, userID int64) error {
	result, err := r.db.Exec(ctx,
		`DELETE FROM files WHERE id = $1 AND user_id = $2`,
		fileID, userID,
	)
	if err != nil {
		return fmt.Errorf("FileRepository.Delete: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("file not found or unauthorized")
	}
	return nil
}

// LinkBlocks inserts file_blocks rows linking ordered block IDs to a file.
func (r *FileRepository) LinkBlocks(ctx context.Context, fileID int64, blockIDs []int64) error {
	batch := &pgxpool.Pool{}
	_ = batch // use direct loop for clarity

	for i, blockID := range blockIDs {
		_, err := r.db.Exec(ctx,
			`INSERT INTO file_blocks (file_id, block_id, block_index) VALUES ($1, $2, $3)`,
			fileID, blockID, i,
		)
		if err != nil {
			return fmt.Errorf("FileRepository.LinkBlocks at index %d: %w", i, err)
		}
	}
	return nil
}

// GetBlockIDs returns block IDs for a file ordered by block_index.
func (r *FileRepository) GetBlockIDs(ctx context.Context, fileID int64) ([]int64, error) {
	rows, err := r.db.Query(ctx,
		`SELECT block_id FROM file_blocks
		 WHERE file_id = $1
		 ORDER BY block_index ASC`,
		fileID,
	)
	if err != nil {
		return nil, fmt.Errorf("FileRepository.GetBlockIDs: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
