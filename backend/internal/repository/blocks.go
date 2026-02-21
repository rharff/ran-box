package repository

import (
	"context"
	"fmt"
	"time"

	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/naratel/naratel-box/backend/internal/logger"
	"github.com/naratel/naratel-box/backend/internal/model"
)

type BlockRepository struct {
	db *pgxpool.Pool
}

func NewBlockRepository(db *pgxpool.Pool) *BlockRepository {
	return &BlockRepository{db: db}
}

// FindByHash returns an existing block by its SHA-256 hash. Returns nil, nil if not found.
func (r *BlockRepository) FindByHash(ctx context.Context, hash string) (*model.Block, error) {
	start := time.Now()
	query := "SELECT id, sha256_hash, s3_key, size_bytes, ref_count, created_at FROM blocks WHERE sha256_hash = $1"

	block := &model.Block{}
	err := r.db.QueryRow(ctx, query, hash,
	).Scan(&block.ID, &block.SHA256Hash, &block.S3Key, &block.SizeBytes, &block.RefCount, &block.CreatedAt)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Info(ctx, "Executed query", logger.QueryAttributes{
				Query: query, DurationMs: duration, RowsAffected: 0,
			})
			return nil, nil
		}
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_QUERY_ERR", Details: fmt.Sprintf("BlockRepository.FindByHash: %s", err.Error()),
		})
		return nil, fmt.Errorf("BlockRepository.FindByHash: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: 1,
	})
	return block, nil
}

// Create inserts a new block record and returns it.
func (r *BlockRepository) Create(ctx context.Context, hash, s3Key string, sizeBytes int64) (*model.Block, error) {
	start := time.Now()
	query := "INSERT INTO blocks (sha256_hash, s3_key, size_bytes, ref_count) VALUES ($1, $2, $3, 1) RETURNING ..."

	block := &model.Block{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO blocks (sha256_hash, s3_key, size_bytes, ref_count)
		 VALUES ($1, $2, $3, 1)
		 RETURNING id, sha256_hash, s3_key, size_bytes, ref_count, created_at`,
		hash, s3Key, sizeBytes,
	).Scan(&block.ID, &block.SHA256Hash, &block.S3Key, &block.SizeBytes, &block.RefCount, &block.CreatedAt)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_INSERT_ERR", Details: fmt.Sprintf("BlockRepository.Create: %s", err.Error()),
		})
		return nil, fmt.Errorf("BlockRepository.Create: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: 1,
	})
	return block, nil
}

// IncrementRefCount increments the reference count for an existing block.
func (r *BlockRepository) IncrementRefCount(ctx context.Context, blockID int64) error {
	start := time.Now()
	query := "UPDATE blocks SET ref_count = ref_count + 1 WHERE id = $1"

	result, err := r.db.Exec(ctx, query, blockID)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_UPDATE_ERR", Details: fmt.Sprintf("BlockRepository.IncrementRefCount: %s", err.Error()),
		})
		return fmt.Errorf("BlockRepository.IncrementRefCount: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: result.RowsAffected(),
	})
	return nil
}

// DecrementRefCount decrements ref_count. Returns the new ref_count.
func (r *BlockRepository) DecrementRefCount(ctx context.Context, blockID int64) (int, error) {
	start := time.Now()
	query := "UPDATE blocks SET ref_count = ref_count - 1 WHERE id = $1 RETURNING ref_count"

	var newCount int
	err := r.db.QueryRow(ctx, query, blockID).Scan(&newCount)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_UPDATE_ERR", Details: fmt.Sprintf("BlockRepository.DecrementRefCount: %s", err.Error()),
		})
		return 0, fmt.Errorf("BlockRepository.DecrementRefCount: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: 1,
	})
	return newCount, nil
}

// Delete permanently removes a block record (call only when ref_count == 0).
func (r *BlockRepository) Delete(ctx context.Context, blockID int64) error {
	start := time.Now()
	query := "DELETE FROM blocks WHERE id = $1"

	result, err := r.db.Exec(ctx, query, blockID)

	duration := time.Since(start).Milliseconds()

	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_DELETE_ERR", Details: fmt.Sprintf("BlockRepository.Delete: %s", err.Error()),
		})
		return fmt.Errorf("BlockRepository.Delete: %w", err)
	}

	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: result.RowsAffected(),
	})
	return nil
}

// FindByIDs returns blocks ordered by the provided ids slice.
func (r *BlockRepository) FindByIDs(ctx context.Context, ids []int64) ([]*model.Block, error) {
	start := time.Now()
	query := "SELECT id, sha256_hash, s3_key, size_bytes, ref_count, created_at FROM blocks WHERE id = ANY($1)"

	rows, err := r.db.Query(ctx, query, ids)
	if err != nil {
		logger.ErrorLog(ctx, "Query failed", logger.ErrorDetails{
			Code: "DB_QUERY_ERR", Details: fmt.Sprintf("BlockRepository.FindByIDs: %s", err.Error()),
		})
		return nil, fmt.Errorf("BlockRepository.FindByIDs: %w", err)
	}
	defer rows.Close()

	blockMap := make(map[int64]*model.Block, len(ids))
	for rows.Next() {
		b := &model.Block{}
		if err := rows.Scan(&b.ID, &b.SHA256Hash, &b.S3Key, &b.SizeBytes, &b.RefCount, &b.CreatedAt); err != nil {
			return nil, err
		}
		blockMap[b.ID] = b
	}

	duration := time.Since(start).Milliseconds()
	logger.Info(ctx, "Executed query", logger.QueryAttributes{
		Query: query, DurationMs: duration, RowsAffected: int64(len(blockMap)),
	})

	// Return in the requested order
	ordered := make([]*model.Block, 0, len(ids))
	for _, id := range ids {
		if b, ok := blockMap[id]; ok {
			ordered = append(ordered, b)
		}
	}
	return ordered, nil
}
