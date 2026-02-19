package model

import "time"

// File represents a file uploaded by a user.
type File struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	MimeType  string    `json:"mime_type"`
	TotalSize int64     `json:"total_size"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FileBlock maps an ordered block to a file.
type FileBlock struct {
	ID         int64 `json:"id"`
	FileID     int64 `json:"file_id"`
	BlockID    int64 `json:"block_id"`
	BlockIndex int   `json:"block_index"` // 0-based order
}
