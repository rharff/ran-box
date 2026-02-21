package block

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sync"

	"github.com/naratel/naratel-box/backend/internal/logger"
	"github.com/naratel/naratel-box/backend/internal/model"
	"github.com/naratel/naratel-box/backend/internal/repository"
	"github.com/naratel/naratel-box/backend/internal/storage"
)

const maxWorkers = 8 // concurrent block upload workers

// blockJob carries a single block's data to a worker.
type blockJob struct {
	index int
	data  []byte
	hash  string
}

// blockResult is the result from a worker after processing a block.
type blockResult struct {
	index   int
	blockID int64
	err     error
}

// Processor handles block splitting, hashing, dedup, and S3 upload.
type Processor struct {
	blockSize  int
	blockRepo  *repository.BlockRepository
	s3         *storage.S3Client
}

// NewProcessor creates a Processor with the given block size in bytes.
func NewProcessor(blockSizeBytes int, blockRepo *repository.BlockRepository, s3 *storage.S3Client) *Processor {
	return &Processor{
		blockSize: blockSizeBytes,
		blockRepo: blockRepo,
		s3:        s3,
	}
}

// Process streams r block-by-block into a worker pool.
// Only maxWorkers blocks are held in memory at any time — O(workers × blockSize)
// memory regardless of total file size, so a 10GB file uses the same RAM as a 10MB file.
func (p *Processor) Process(ctx context.Context, r io.Reader) ([]int64, int64, error) {
	// jobCh is bounded to maxWorkers so the reader blocks when all workers are busy,
	// preventing unbounded memory growth.
	jobCh    := make(chan blockJob, maxWorkers)
	resultCh := make(chan blockResult, maxWorkers)

	// Start the fixed worker pool.
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				blockID, err := p.processBlock(ctx, job)
				resultCh <- blockResult{index: job.index, blockID: blockID, err: err}
			}
		}()
	}

	// Close resultCh once all workers finish.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Read the file one block at a time and feed workers.
	// This goroutine blocks on jobCh when all workers are busy, keeping memory bounded.
	var totalBytes int64
	var readErr   error
	go func() {
		defer close(jobCh)
		buf   := make([]byte, p.blockSize)
		index := 0
		for {
			n, err := io.ReadFull(r, buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				totalBytes += int64(n)
				jobCh <- blockJob{index: index, data: data, hash: sha256Block(data)}
				index++
			}
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			if err != nil {
				readErr = fmt.Errorf("splitStream read error: %w", err)
				return
			}
		}
	}()

	// Collect results and preserve order.
	var results []blockResult
	for res := range resultCh {
		if res.err != nil {
			return nil, 0, fmt.Errorf("worker error at block %d: %w", res.index, res.err)
		}
		results = append(results, res)
	}

	if readErr != nil {
		return nil, 0, readErr
	}

	ordered := make([]int64, len(results))
	for _, res := range results {
		ordered[res.index] = res.blockID
	}
	return ordered, totalBytes, nil
}

// processBlock handles one block: check dedup → upload if new → return block ID.
func (p *Processor) processBlock(ctx context.Context, job blockJob) (int64, error) {
	// Check dedup: does this hash already exist?
	existing, err := p.blockRepo.FindByHash(ctx, job.hash)
	if err != nil {
		return 0, fmt.Errorf("processBlock FindByHash: %w", err)
	}

	if existing != nil {
		// ── DEDUP HIT: skip upload, just bump ref count ──
		if err := p.blockRepo.IncrementRefCount(ctx, existing.ID); err != nil {
			return 0, err
		}
		logger.Info(ctx, "Block deduplication hit", map[string]interface{}{
			"block_index": job.index, "block_id": existing.ID, "hash": job.hash, "size_bytes": len(job.data),
		})
		return existing.ID, nil
	}

	// ── NEW BLOCK: upload to S3 then register in DB ──
	s3Key := job.hash // S3 object key == SHA-256 hex
	if err := p.s3.PutObject(ctx, s3Key, bytes.NewReader(job.data), int64(len(job.data))); err != nil {
		logger.ErrorLog(ctx, "Block S3 upload failed", logger.ErrorDetails{
			Code: "S3_PUT_ERR", Details: fmt.Sprintf("index=%d hash=%s: %s", job.index, job.hash, err.Error()),
		})
		return 0, fmt.Errorf("processBlock PutObject: %w", err)
	}

	newBlock, err := p.blockRepo.Create(ctx, job.hash, s3Key, int64(len(job.data)))
	if err != nil {
		return 0, fmt.Errorf("processBlock Create block record: %w", err)
	}

	logger.Info(ctx, "New block uploaded to S3", map[string]interface{}{
		"block_index": job.index, "block_id": newBlock.ID, "hash": job.hash, "size_bytes": len(job.data),
	})

	return newBlock.ID, nil
}

// sha256Block returns the hex-encoded SHA-256 hash of data.
func sha256Block(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// BlocksToStream fetches blocks from S3 in order and writes them to w.
func BlocksToStream(ctx context.Context, blocks []*model.Block, s3 *storage.S3Client, w io.Writer) error {
	for _, b := range blocks {
		body, err := s3.GetObject(ctx, b.S3Key)
		if err != nil {
			logger.ErrorLog(ctx, "Block stream S3 fetch failed", logger.ErrorDetails{
				Code: "S3_GET_ERR", Details: fmt.Sprintf("s3_key=%s: %s", b.S3Key, err.Error()),
			})
			return fmt.Errorf("BlocksToStream GetObject key=%s: %w", b.S3Key, err)
		}
		_, copyErr := io.Copy(w, body)
		body.Close()
		if copyErr != nil {
			logger.ErrorLog(ctx, "Block stream copy failed", logger.ErrorDetails{
				Code: "STREAM_COPY_ERR", Details: fmt.Sprintf("s3_key=%s: %s", b.S3Key, copyErr.Error()),
			})
			return fmt.Errorf("BlocksToStream io.Copy key=%s: %w", b.S3Key, copyErr)
		}
	}
	return nil
}
