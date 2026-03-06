package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Artifact represents a stored build artifact.
type Artifact struct {
	ID           uuid.UUID       `json:"id" db:"id"`
	ProjectID    uuid.UUID       `json:"project_id" db:"project_id"`
	RunID        uuid.UUID       `json:"run_id" db:"run_id"`
	TaskName     *string         `json:"task_name,omitempty" db:"task_name"`
	Name         string          `json:"name" db:"name"`
	FileName     string          `json:"file_name" db:"file_name"`
	ContentType  string          `json:"content_type" db:"content_type"`
	SizeBytes    int64           `json:"size_bytes" db:"size_bytes"`
	StorageKey   string          `json:"storage_key" db:"storage_key"`
	DigestSHA256 *string         `json:"digest_sha256,omitempty" db:"digest_sha256"`
	ExpiresAt    *time.Time      `json:"expires_at,omitempty" db:"expires_at"`
	Metadata     json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
}
