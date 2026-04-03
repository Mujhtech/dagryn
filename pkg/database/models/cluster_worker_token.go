package models

import (
	"time"

	"github.com/google/uuid"
)

type ClusterWorkerTokenScope string

const (
	ClusterWorkerTokenScopeTeam     ClusterWorkerTokenScope = "team"
	ClusterWorkerTokenScopePersonal ClusterWorkerTokenScope = "personal"
)

type ClusterWorkerToken struct {
	ID              uuid.UUID               `json:"id" db:"id"`
	Name            string                  `json:"name" db:"name"`
	KeyHash         string                  `json:"-" db:"key_hash"`
	KeyPrefix       string                  `json:"key_prefix" db:"key_prefix"`
	ScopeType       ClusterWorkerTokenScope `json:"scope_type" db:"scope_type"`
	TeamID          *uuid.UUID              `json:"team_id,omitempty" db:"team_id"`
	OwnerUserID     *uuid.UUID              `json:"owner_user_id,omitempty" db:"owner_user_id"`
	ClusterID       *uuid.UUID              `json:"cluster_id,omitempty" db:"cluster_id"`
	LastUsedAt      *time.Time              `json:"last_used_at,omitempty" db:"last_used_at"`
	ExpiresAt       *time.Time              `json:"expires_at,omitempty" db:"expires_at"`
	CreatedByUserID uuid.UUID               `json:"created_by_user_id" db:"created_by_user_id"`
	CreatedAt       time.Time               `json:"created_at" db:"created_at"`
	RevokedAt       *time.Time              `json:"revoked_at,omitempty" db:"revoked_at"`
}
