package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// PluginPublisher represents a plugin publisher/organization.
type PluginPublisher struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	Name        string     `json:"name" db:"name"` // unique slug, e.g. "dagryn"
	DisplayName string     `json:"display_name" db:"display_name"`
	AvatarURL   *string    `json:"avatar_url,omitempty" db:"avatar_url"`
	Website     *string    `json:"website,omitempty" db:"website"`
	Verified    bool       `json:"verified" db:"verified"`
	UserID      *uuid.UUID `json:"user_id,omitempty" db:"user_id"` // owning user
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// RegistryPlugin represents a plugin in the registry.
type RegistryPlugin struct {
	ID              uuid.UUID `json:"id" db:"id"`
	PublisherID     uuid.UUID `json:"publisher_id" db:"publisher_id"`
	Name            string    `json:"name" db:"name"`
	Description     string    `json:"description" db:"description"`
	Type            string    `json:"type" db:"type"` // "tool", "composite", "integration"
	License         *string   `json:"license,omitempty" db:"license"`
	Homepage        *string   `json:"homepage,omitempty" db:"homepage"`
	RepositoryURL   *string   `json:"repository_url,omitempty" db:"repository_url"`
	Readme          *string   `json:"readme,omitempty" db:"readme"`
	TotalDownloads  int64     `json:"total_downloads" db:"total_downloads"`
	WeeklyDownloads int64     `json:"weekly_downloads" db:"weekly_downloads"`
	Stars           int       `json:"stars" db:"stars"`
	Featured        bool      `json:"featured" db:"featured"`
	Deprecated      bool      `json:"deprecated" db:"deprecated"`
	LatestVersion   string    `json:"latest_version" db:"latest_version"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// RegistryPluginWithPublisher includes publisher info.
type RegistryPluginWithPublisher struct {
	RegistryPlugin
	PublisherName     string `json:"publisher_name" db:"publisher_name"`
	PublisherVerified bool   `json:"publisher_verified" db:"publisher_verified"`
}

// PluginVersion represents a specific version of a registry plugin.
type PluginVersion struct {
	ID             uuid.UUID       `json:"id" db:"id"`
	PluginID       uuid.UUID       `json:"plugin_id" db:"plugin_id"`
	Version        string          `json:"version" db:"version"`
	ManifestJSON   json.RawMessage `json:"manifest_json" db:"manifest_json"`
	ChecksumSHA256 *string         `json:"checksum_sha256,omitempty" db:"checksum_sha256"`
	Downloads      int64           `json:"downloads" db:"downloads"`
	Yanked         bool            `json:"yanked" db:"yanked"`
	ReleaseNotes   *string         `json:"release_notes,omitempty" db:"release_notes"`
	PublishedAt    time.Time       `json:"published_at" db:"published_at"`
}

// PluginDownload records a single download event.
type PluginDownload struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	PluginID  uuid.UUID  `json:"plugin_id" db:"plugin_id"`
	VersionID uuid.UUID  `json:"version_id" db:"version_id"`
	UserID    *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	IPHash    *string    `json:"ip_hash,omitempty" db:"ip_hash"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}
