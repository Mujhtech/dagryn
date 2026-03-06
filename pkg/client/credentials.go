package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Credentials holds authentication tokens.
type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	UserID       uuid.UUID `json:"user_id"`
	UserEmail    string    `json:"user_email"`
	ServerURL    string    `json:"server_url"`
}

// IsExpired returns true if the access token has expired.
func (c *Credentials) IsExpired() bool {
	// Add a 1 minute buffer for clock skew
	return time.Now().Add(time.Minute).After(c.ExpiresAt)
}

// CredentialsStore manages credential storage.
type CredentialsStore struct {
	configDir string
}

// NewCredentialsStore creates a new credentials store.
func NewCredentialsStore() (*CredentialsStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".dagryn")
	return &CredentialsStore{configDir: configDir}, nil
}

// credentialsPath returns the path to the credentials file.
func (s *CredentialsStore) credentialsPath() string {
	return filepath.Join(s.configDir, "credentials.json")
}

// Save saves credentials to disk.
func (s *CredentialsStore) Save(creds *Credentials) error {
	// Ensure config directory exists
	if err := os.MkdirAll(s.configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Write with restricted permissions
	if err := os.WriteFile(s.credentialsPath(), data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}

	return nil
}

// Load loads credentials from disk.
func (s *CredentialsStore) Load() (*Credentials, error) {
	data, err := os.ReadFile(s.credentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No credentials stored
		}
		return nil, fmt.Errorf("failed to read credentials: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return &creds, nil
}

// Delete removes stored credentials.
func (s *CredentialsStore) Delete() error {
	path := s.credentialsPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Already deleted
	}
	return os.Remove(path)
}

// Exists returns true if credentials exist.
func (s *CredentialsStore) Exists() bool {
	_, err := os.Stat(s.credentialsPath())
	return err == nil
}

// ConfigDir returns the configuration directory path.
func (s *CredentialsStore) ConfigDir() string {
	return s.configDir
}
