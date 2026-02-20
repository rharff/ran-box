package model

import "time"

// ShareLink represents a public share link for a file.
type ShareLink struct {
	ID        int64      `json:"id"`
	FileID    int64      `json:"file_id"`
	UserID    int64      `json:"user_id"`
	Token     string     `json:"token"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
