package model

import "time"

// Block represents a deduplicated chunk of file data stored in S3.
type Block struct {
	ID         int64     `json:"id"`
	SHA256Hash string    `json:"sha256_hash"` // hex-encoded, also used as S3 key
	S3Key      string    `json:"s3_key"`
	SizeBytes  int64     `json:"size_bytes"`
	RefCount   int       `json:"ref_count"`
	CreatedAt  time.Time `json:"created_at"`
}
