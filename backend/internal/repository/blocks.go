package repository

import (
	"context"
	"fmt"

	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	block := &model.Block{}
	err := r.db.QueryRow(ctx,
		`SELECT id, sha256_hash, s3_key, size_bytes, ref_count, created_at
		 FROM blocks WHERE sha256_hash = $1`,
		hash,
	).Scan(&block.ID, &block.SHA256Hash, &block.S3Key, &block.SizeBytes, &block.RefCount, &block.CreatedAt)
	if err != nil {
		// pgx returns pgx.ErrNoRows when no row found
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("BlockRepository.FindByHash: %w", err)
	}
	return block, nil
}

// Create inserts a new block record and returns it.
func (r *BlockRepository) Create(ctx context.Context, hash, s3Key string, sizeBytes int64) (*model.Block, error) {
	block := &model.Block{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO blocks (sha256_hash, s3_key, size_bytes, ref_count)
		 VALUES ($1, $2, $3, 1)
		 RETURNING id, sha256_hash, s3_key, size_bytes, ref_count, created_at`,
		hash, s3Key, sizeBytes,
	).Scan(&block.ID, &block.SHA256Hash, &block.S3Key, &block.SizeBytes, &block.RefCount, &block.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("BlockRepository.Create: %w", err)
	}
	return block, nil
}

// IncrementRefCount increments the reference count for an existing block.
func (r *BlockRepository) IncrementRefCount(ctx context.Context, blockID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE blocks SET ref_count = ref_count + 1 WHERE id = $1`,
		blockID,
	)
	if err != nil {
		return fmt.Errorf("BlockRepository.IncrementRefCount: %w", err)
	}
	return nil
}

// DecrementRefCount decrements ref_count. Returns the new ref_count.
func (r *BlockRepository) DecrementRefCount(ctx context.Context, blockID int64) (int, error) {
	var newCount int
	err := r.db.QueryRow(ctx,
		`UPDATE blocks SET ref_count = ref_count - 1 WHERE id = $1 RETURNING ref_count`,
		blockID,
	).Scan(&newCount)
	if err != nil {
		return 0, fmt.Errorf("BlockRepository.DecrementRefCount: %w", err)
	}
	return newCount, nil
}

// Delete permanently removes a block record (call only when ref_count == 0).
func (r *BlockRepository) Delete(ctx context.Context, blockID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM blocks WHERE id = $1`, blockID)
	if err != nil {
		return fmt.Errorf("BlockRepository.Delete: %w", err)
	}
	return nil
}

// FindByIDs returns blocks ordered by the provided ids slice.
func (r *BlockRepository) FindByIDs(ctx context.Context, ids []int64) ([]*model.Block, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, sha256_hash, s3_key, size_bytes, ref_count, created_at
		 FROM blocks WHERE id = ANY($1)`,
		ids,
	)
	if err != nil {
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

	// Return in the requested order
	ordered := make([]*model.Block, 0, len(ids))
	for _, id := range ids {
		if b, ok := blockMap[id]; ok {
			ordered = append(ordered, b)
		}
	}
	return ordered, nil
}
