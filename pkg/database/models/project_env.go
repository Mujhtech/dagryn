package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type EnvValueType string

const (
	EnvValueTypePlain  EnvValueType = "plain"
	EnvValueTypeSecret EnvValueType = "secret"
)

type SecretProvider string

const (
	SecretProviderDB         SecretProvider = "db"
	SecretProviderAWS        SecretProvider = "aws_sm"
	SecretProviderGCP        SecretProvider = "gcp_sm"
	SecretProviderCloudflare SecretProvider = "cloudflare"
)

type SecretRecordStatus string

const (
	SecretRecordStatusActive  SecretRecordStatus = "active"
	SecretRecordStatusRotated SecretRecordStatus = "rotated"
	SecretRecordStatusRevoked SecretRecordStatus = "revoked"
)

type ProjectEnvVar struct {
	ID             uuid.UUID      `json:"id" db:"id"`
	ProjectID      uuid.UUID      `json:"project_id" db:"project_id"`
	Key            string         `json:"key" db:"key"`
	ValueType      EnvValueType   `json:"value_type" db:"value_type"`
	PlainValue     *string        `json:"plain_value,omitempty" db:"plain_value"`
	Environment    *string        `json:"environment,omitempty" db:"environment"`
	Branch         *string        `json:"branch,omitempty" db:"branch"`
	Required       bool           `json:"required" db:"required"`
	Enabled        bool           `json:"enabled" db:"enabled"`
	Provider       SecretProvider `json:"provider" db:"provider"`
	ProviderRef    *string        `json:"provider_ref,omitempty" db:"provider_ref"`
	SecretRecordID *uuid.UUID     `json:"secret_record_id,omitempty" db:"secret_record_id"`
	Description    *string        `json:"description,omitempty" db:"description"`
	CreatedBy      *uuid.UUID     `json:"created_by,omitempty" db:"created_by"`
	UpdatedBy      *uuid.UUID     `json:"updated_by,omitempty" db:"updated_by"`
	CreatedAt      time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at" db:"updated_at"`
	DeletedAt      *time.Time     `json:"deleted_at,omitempty" db:"deleted_at"`
}

type SecretRecord struct {
	ID          uuid.UUID          `json:"id" db:"id"`
	Provider    SecretProvider     `json:"provider" db:"provider"`
	Ciphertext  []byte             `json:"-" db:"ciphertext"`
	KeyRef      *string            `json:"key_ref,omitempty" db:"key_ref"`
	Checksum    *string            `json:"checksum,omitempty" db:"checksum"`
	Version     *string            `json:"version,omitempty" db:"version"`
	ExternalRef *string            `json:"external_ref,omitempty" db:"external_ref"`
	Status      SecretRecordStatus `json:"status" db:"status"`
	CreatedAt   time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" db:"updated_at"`
	RotatedAt   *time.Time         `json:"rotated_at,omitempty" db:"rotated_at"`
}

type EnvAuditEvent struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	ProjectID   uuid.UUID       `json:"project_id" db:"project_id"`
	EnvVarID    *uuid.UUID      `json:"env_var_id,omitempty" db:"env_var_id"`
	ActorID     *uuid.UUID      `json:"actor_id,omitempty" db:"actor_id"`
	Action      string          `json:"action" db:"action"`
	Provider    *SecretProvider `json:"provider,omitempty" db:"provider"`
	Environment *string         `json:"environment,omitempty" db:"environment"`
	Branch      *string         `json:"branch,omitempty" db:"branch"`
	Metadata    json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}
