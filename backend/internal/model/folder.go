package model

import "time"

// Folder represents a directory in the user's file system.
type Folder struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	ParentID  *int64    `json:"parent_id"` // nil = root level
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
