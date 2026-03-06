package licensing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// StoredLicense holds local license data persisted in ~/.dagryn/license.json.
type StoredLicense struct {
	Key          string    `json:"key,omitempty"`
	LicenseID    string    `json:"license_id,omitempty"`
	InstanceID   string    `json:"instance_id,omitempty"`
	InstanceName string    `json:"instance_name,omitempty"`
	ActivatedAt  time.Time `json:"activated_at,omitempty"`
}

// LicensePath returns the path to the stored license file (~/.dagryn/license.json).
func LicensePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".dagryn", "license.json"), nil
}

// LoadStoredLicense reads the stored license from disk.
func LoadStoredLicense() (StoredLicense, error) {
	var stored StoredLicense
	path, err := LicensePath()
	if err != nil {
		return stored, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return stored, err
	}
	err = json.Unmarshal(data, &stored)
	return stored, err
}

// SaveStoredLicense writes the stored license to disk.
func SaveStoredLicense(stored StoredLicense) error {
	path, err := LicensePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// RemoveStoredLicense deletes the stored license from disk.
func RemoveStoredLicense() error {
	path, err := LicensePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
