package block

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sync"

	"github.com/naratel/naratel-box/backend/internal/model"
	"github.com/naratel/naratel-box/backend/internal/repository"
	"github.com/naratel/naratel-box/backend/internal/storage"
)

const maxWorkers = 4 // concurrent block upload workers

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

// Process reads the full file from r, splits it into blocks, deduplicates,
// uploads new blocks to S3, and returns ordered block IDs and total bytes read.
func (p *Processor) Process(ctx context.Context, r io.Reader) ([]int64, int64, error) {
	// Read and split the stream into block-sized chunks
	jobs, totalBytes, err := p.splitStream(r)
	if err != nil {
		return nil, 0, fmt.Errorf("Processor.splitStream: %w", err)
	}

	// Process blocks concurrently with a worker pool
	results, err := p.runWorkers(ctx, jobs)
	if err != nil {
		return nil, 0, err
	}

	// Sort results by index to preserve block order
	ordered := make([]int64, len(results))
	for _, res := range results {
		ordered[res.index] = res.blockID
	}

	return ordered, totalBytes, nil
}

// splitStream reads r in blockSize chunks and returns a slice of blockJobs.
func (p *Processor) splitStream(r io.Reader) ([]blockJob, int64, error) {
	var jobs []blockJob
	var totalBytes int64
	index := 0

	buf := make([]byte, p.blockSize)
	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])

			hash := sha256Block(data)
			jobs = append(jobs, blockJob{
				index: index,
				data:  data,
				hash:  hash,
			})
			totalBytes += int64(n)
			index++
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("splitStream read error: %w", err)
		}
	}
	return jobs, totalBytes, nil
}

// runWorkers fans out block jobs to a pool of workers and collects results.
func (p *Processor) runWorkers(ctx context.Context, jobs []blockJob) ([]blockResult, error) {
	jobCh := make(chan blockJob, len(jobs))
	resultCh := make(chan blockResult, len(jobs))

	// Start workers
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

	// Send all jobs
	for _, job := range jobs {
		jobCh <- job
	}
	close(jobCh)

	// Wait then close results
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results, fail fast on first error
	var results []blockResult
	for res := range resultCh {
		if res.err != nil {
			return nil, fmt.Errorf("worker error at block %d: %w", res.index, res.err)
		}
		results = append(results, res)
	}
	return results, nil
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
		return existing.ID, nil
	}

	// ── NEW BLOCK: upload to S3 then register in DB ──
	s3Key := job.hash // S3 object key == SHA-256 hex
	if err := p.s3.PutObject(ctx, s3Key, bytes.NewReader(job.data), int64(len(job.data))); err != nil {
		return 0, fmt.Errorf("processBlock PutObject: %w", err)
	}

	newBlock, err := p.blockRepo.Create(ctx, job.hash, s3Key, int64(len(job.data)))
	if err != nil {
		return 0, fmt.Errorf("processBlock Create block record: %w", err)
	}

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
			return fmt.Errorf("BlocksToStream GetObject key=%s: %w", b.S3Key, err)
		}
		_, copyErr := io.Copy(w, body)
		body.Close()
		if copyErr != nil {
			return fmt.Errorf("BlocksToStream io.Copy key=%s: %w", b.S3Key, copyErr)
		}
	}
	return nil
}
