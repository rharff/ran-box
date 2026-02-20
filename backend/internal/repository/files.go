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
func (r *FileRepository) Create(ctx context.Context, userID int64, name, mimeType string, totalSize int64, folderID *int64) (*model.File, error) {
	file := &model.File{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO files (user_id, name, mime_type, total_size, folder_id)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, user_id, folder_id, name, mime_type, total_size, created_at, updated_at`,
		userID, name, mimeType, totalSize, folderID,
	).Scan(&file.ID, &file.UserID, &file.FolderID, &file.Name, &file.MimeType, &file.TotalSize, &file.CreatedAt, &file.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("FileRepository.Create: %w", err)
	}
	return file, nil
}

// FindByIDAndUserID fetches a file only if it belongs to the given user (ownership check).
func (r *FileRepository) FindByIDAndUserID(ctx context.Context, fileID, userID int64) (*model.File, error) {
	file := &model.File{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, folder_id, name, mime_type, total_size, created_at, updated_at
		 FROM files WHERE id = $1 AND user_id = $2`,
		fileID, userID,
	).Scan(&file.ID, &file.UserID, &file.FolderID, &file.Name, &file.MimeType, &file.TotalSize, &file.CreatedAt, &file.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("FileRepository.FindByIDAndUserID: %w", err)
	}
	return file, nil
}

// FindByID fetches a file by ID regardless of ownership (for share links).
func (r *FileRepository) FindByID(ctx context.Context, fileID int64) (*model.File, error) {
	file := &model.File{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, folder_id, name, mime_type, total_size, created_at, updated_at
		 FROM files WHERE id = $1`,
		fileID,
	).Scan(&file.ID, &file.UserID, &file.FolderID, &file.Name, &file.MimeType, &file.TotalSize, &file.CreatedAt, &file.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("FileRepository.FindByID: %w", err)
	}
	return file, nil
}

// ListByUserID returns all files for a user ordered by newest first.
func (r *FileRepository) ListByUserID(ctx context.Context, userID int64) ([]*model.File, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, folder_id, name, mime_type, total_size, created_at, updated_at
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
		if err := rows.Scan(&f.ID, &f.UserID, &f.FolderID, &f.Name, &f.MimeType, &f.TotalSize, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// ListByFolder returns files in a specific folder (or root if folderID is nil).
func (r *FileRepository) ListByFolder(ctx context.Context, userID int64, folderID *int64) ([]*model.File, error) {
	var rows interface{ Next() bool; Scan(dest ...interface{}) error; Close() }
	var err error

	if folderID == nil {
		rows2, err2 := r.db.Query(ctx,
			`SELECT id, user_id, folder_id, name, mime_type, total_size, created_at, updated_at
			 FROM files WHERE user_id = $1 AND folder_id IS NULL
			 ORDER BY name ASC`,
			userID,
		)
		if err2 != nil {
			return nil, fmt.Errorf("FileRepository.ListByFolder: %w", err2)
		}
		rows = rows2
		defer rows2.Close()
	} else {
		rows2, err2 := r.db.Query(ctx,
			`SELECT id, user_id, folder_id, name, mime_type, total_size, created_at, updated_at
			 FROM files WHERE user_id = $1 AND folder_id = $2
			 ORDER BY name ASC`,
			userID, *folderID,
		)
		if err2 != nil {
			return nil, fmt.Errorf("FileRepository.ListByFolder: %w", err2)
		}
		rows = rows2
		defer rows2.Close()
	}
	_ = err

	var files []*model.File
	for rows.Next() {
		f := &model.File{}
		if err := rows.Scan(&f.ID, &f.UserID, &f.FolderID, &f.Name, &f.MimeType, &f.TotalSize, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// Search searches files by name for a given user.
func (r *FileRepository) Search(ctx context.Context, userID int64, query string) ([]*model.File, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, folder_id, name, mime_type, total_size, created_at, updated_at
		 FROM files WHERE user_id = $1 AND LOWER(name) LIKE '%' || LOWER($2) || '%'
		 ORDER BY name ASC
		 LIMIT 50`,
		userID, query,
	)
	if err != nil {
		return nil, fmt.Errorf("FileRepository.Search: %w", err)
	}
	defer rows.Close()

	var files []*model.File
	for rows.Next() {
		f := &model.File{}
		if err := rows.Scan(&f.ID, &f.UserID, &f.FolderID, &f.Name, &f.MimeType, &f.TotalSize, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// Rename updates the name of a file.
func (r *FileRepository) Rename(ctx context.Context, fileID, userID int64, newName string) (*model.File, error) {
	file := &model.File{}
	err := r.db.QueryRow(ctx,
		`UPDATE files SET name = $1, updated_at = NOW()
		 WHERE id = $2 AND user_id = $3
		 RETURNING id, user_id, folder_id, name, mime_type, total_size, created_at, updated_at`,
		newName, fileID, userID,
	).Scan(&file.ID, &file.UserID, &file.FolderID, &file.Name, &file.MimeType, &file.TotalSize, &file.CreatedAt, &file.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("FileRepository.Rename: %w", err)
	}
	return file, nil
}

// Move updates the folder_id of a file.
func (r *FileRepository) Move(ctx context.Context, fileID, userID int64, folderID *int64) (*model.File, error) {
	file := &model.File{}
	err := r.db.QueryRow(ctx,
		`UPDATE files SET folder_id = $1, updated_at = NOW()
		 WHERE id = $2 AND user_id = $3
		 RETURNING id, user_id, folder_id, name, mime_type, total_size, created_at, updated_at`,
		folderID, fileID, userID,
	).Scan(&file.ID, &file.UserID, &file.FolderID, &file.Name, &file.MimeType, &file.TotalSize, &file.CreatedAt, &file.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("FileRepository.Move: %w", err)
	}
	return file, nil
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
