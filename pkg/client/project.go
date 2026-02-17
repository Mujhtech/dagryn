package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// ProjectConfig holds the local project configuration linking to a remote project.
type ProjectConfig struct {
	ProjectID   uuid.UUID `json:"project_id"`
	ServerURL   string    `json:"server_url"`
	ProjectName string    `json:"project_name"`
	ProjectSlug string    `json:"project_slug"`
	TeamID      uuid.UUID `json:"team_id,omitempty"`
	TeamName    string    `json:"team_name,omitempty"`
	LinkedAt    time.Time `json:"linked_at"`
}

// ProjectConfigStore manages project configuration storage in .dagryn/project.json
type ProjectConfigStore struct {
	projectRoot string
}

// NewProjectConfigStore creates a new project config store for the given project root.
func NewProjectConfigStore(projectRoot string) *ProjectConfigStore {
	return &ProjectConfigStore{projectRoot: projectRoot}
}

// configDir returns the .dagryn directory path.
func (s *ProjectConfigStore) configDir() string {
	return filepath.Join(s.projectRoot, ".dagryn")
}

// configPath returns the path to the project.json file.
func (s *ProjectConfigStore) configPath() string {
	return filepath.Join(s.configDir(), "project.json")
}

// Save saves the project configuration to disk.
func (s *ProjectConfigStore) Save(cfg *ProjectConfig) error {
	// Ensure .dagryn directory exists
	if err := os.MkdirAll(s.configDir(), 0755); err != nil {
		return fmt.Errorf("failed to create .dagryn directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}

	if err := os.WriteFile(s.configPath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write project config: %w", err)
	}

	return nil
}

// Load loads the project configuration from disk.
// Returns nil, nil if the config file doesn't exist.
func (s *ProjectConfigStore) Load() (*ProjectConfig, error) {
	data, err := os.ReadFile(s.configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No config stored
		}
		return nil, fmt.Errorf("failed to read project config: %w", err)
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse project config: %w", err)
	}

	return &cfg, nil
}

// Delete removes the stored project configuration.
func (s *ProjectConfigStore) Delete() error {
	path := s.configPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Already deleted
	}
	return os.Remove(path)
}

// Exists returns true if project configuration exists.
func (s *ProjectConfigStore) Exists() bool {
	_, err := os.Stat(s.configPath())
	return err == nil
}

// ProjectRoot returns the project root directory.
func (s *ProjectConfigStore) ProjectRoot() string {
	return s.projectRoot
}

// ConfigDir returns the .dagryn configuration directory path.
func (s *ProjectConfigStore) ConfigDir() string {
	return s.configDir()
}
