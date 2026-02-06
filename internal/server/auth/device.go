package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrDeviceCodeNotFound is returned when a device code is not found.
	ErrDeviceCodeNotFound = errors.New("device code not found")
	// ErrDeviceCodeExpired is returned when a device code has expired.
	ErrDeviceCodeExpired = errors.New("device code expired")
	// ErrDeviceCodeAlreadyUsed is returned when a device code has already been used.
	ErrDeviceCodeAlreadyUsed = errors.New("device code already used")
	// ErrDeviceCodePending is returned when authorization is still pending.
	ErrDeviceCodePending = errors.New("authorization pending")
)

// DeviceCodeConfig holds device code flow configuration.
type DeviceCodeConfig struct {
	// Expiry is how long device codes are valid.
	Expiry time.Duration
	// PollInterval is the minimum interval between poll requests.
	PollInterval time.Duration
	// VerificationURI is the URL users visit to authorize.
	VerificationURI string
}

// DefaultDeviceCodeConfig returns sensible defaults.
func DefaultDeviceCodeConfig() DeviceCodeConfig {
	return DeviceCodeConfig{
		Expiry:          15 * time.Minute,
		PollInterval:    5 * time.Second,
		VerificationURI: "http://localhost:9000/device",
	}
}

// DeviceCodeRecord represents a device code in the database.
type DeviceCodeRecord struct {
	ID              uuid.UUID
	DeviceCode      string
	UserCode        string
	UserID          *uuid.UUID // Set when user authorizes
	ExpiresAt       time.Time
	AuthorizedAt    *time.Time
	PollInterval    int
	VerificationURI string
	CreatedAt       time.Time
}

// DeviceCodeResponse is returned when a device code is requested.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// DeviceCodeService handles the device code flow for CLI authentication.
type DeviceCodeService struct {
	pool   *pgxpool.Pool
	config DeviceCodeConfig
}

// NewDeviceCodeService creates a new device code service.
func NewDeviceCodeService(pool *pgxpool.Pool, config DeviceCodeConfig) *DeviceCodeService {
	return &DeviceCodeService{
		pool:   pool,
		config: config,
	}
}

// RequestDeviceCode creates a new device code for CLI authentication.
func (s *DeviceCodeService) RequestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
	// Generate codes
	deviceCode, err := GenerateDeviceCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate device code: %w", err)
	}

	userCode, err := GenerateUserCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate user code: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(s.config.Expiry)
	id := uuid.New()

	// Store in database
	_, err = s.pool.Exec(ctx, `
		INSERT INTO device_codes (id, device_code, user_code, expires_at, poll_interval, verification_uri, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, id, deviceCode, userCode, expiresAt, int(s.config.PollInterval.Seconds()), s.config.VerificationURI, now)
	if err != nil {
		return nil, fmt.Errorf("failed to store device code: %w", err)
	}

	return &DeviceCodeResponse{
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		VerificationURI: s.config.VerificationURI,
		ExpiresIn:       int(s.config.Expiry.Seconds()),
		Interval:        int(s.config.PollInterval.Seconds()),
	}, nil
}

// GetByUserCode retrieves a device code record by user code.
func (s *DeviceCodeService) GetByUserCode(ctx context.Context, userCode string) (*DeviceCodeRecord, error) {
	var record DeviceCodeRecord
	err := s.pool.QueryRow(ctx, `
		SELECT id, device_code, user_code, user_id, expires_at, authorized_at, poll_interval, verification_uri, created_at
		FROM device_codes
		WHERE user_code = $1
	`, userCode).Scan(
		&record.ID, &record.DeviceCode, &record.UserCode, &record.UserID,
		&record.ExpiresAt, &record.AuthorizedAt, &record.PollInterval,
		&record.VerificationURI, &record.CreatedAt,
	)

	if err != nil {
		log.Println("Error getting device code by user code:", err)
		return nil, ErrDeviceCodeNotFound
	}

	if time.Now().After(record.ExpiresAt) {
		return nil, ErrDeviceCodeExpired
	}

	return &record, nil
}

// GetByDeviceCode retrieves a device code record by device code.
func (s *DeviceCodeService) GetByDeviceCode(ctx context.Context, deviceCode string) (*DeviceCodeRecord, error) {
	var record DeviceCodeRecord
	err := s.pool.QueryRow(ctx, `
		SELECT id, device_code, user_code, user_id, expires_at, authorized_at, poll_interval, verification_uri, created_at
		FROM device_codes
		WHERE device_code = $1
	`, deviceCode).Scan(
		&record.ID, &record.DeviceCode, &record.UserCode, &record.UserID,
		&record.ExpiresAt, &record.AuthorizedAt, &record.PollInterval,
		&record.VerificationURI, &record.CreatedAt,
	)
	if err != nil {
		return nil, ErrDeviceCodeNotFound
	}

	if time.Now().After(record.ExpiresAt) {
		return nil, ErrDeviceCodeExpired
	}

	return &record, nil
}

// Authorize marks a device code as authorized by a user.
// This is called when the user approves the device in the browser.
func (s *DeviceCodeService) Authorize(ctx context.Context, userCode string, userID uuid.UUID) error {
	// Check if code exists and is valid
	record, err := s.GetByUserCode(ctx, userCode)
	if err != nil {
		return err
	}

	if record.AuthorizedAt != nil {
		return ErrDeviceCodeAlreadyUsed
	}

	// Mark as authorized
	now := time.Now()
	result, err := s.pool.Exec(ctx, `
		UPDATE device_codes
		SET user_id = $1, authorized_at = $2
		WHERE user_code = $3 AND authorized_at IS NULL AND expires_at > NOW()
	`, userID, now, userCode)
	if err != nil {
		return fmt.Errorf("failed to authorize device code: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrDeviceCodeAlreadyUsed
	}

	return nil
}

// Poll checks if a device code has been authorized.
// Returns the user ID if authorized, or an error if pending/expired.
func (s *DeviceCodeService) Poll(ctx context.Context, deviceCode string) (*uuid.UUID, error) {
	record, err := s.GetByDeviceCode(ctx, deviceCode)
	if err != nil {
		return nil, err
	}

	if record.AuthorizedAt == nil {
		return nil, ErrDeviceCodePending
	}

	return record.UserID, nil
}

// Consume marks a device code as consumed (used to generate tokens).
// After this, the device code cannot be used again.
func (s *DeviceCodeService) Consume(ctx context.Context, deviceCode string) error {
	result, err := s.pool.Exec(ctx, `
		DELETE FROM device_codes
		WHERE device_code = $1 AND authorized_at IS NOT NULL
	`, deviceCode)
	if err != nil {
		return fmt.Errorf("failed to consume device code: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrDeviceCodeNotFound
	}

	return nil
}

// Cleanup removes expired device codes.
func (s *DeviceCodeService) Cleanup(ctx context.Context) (int64, error) {
	result, err := s.pool.Exec(ctx, `
		DELETE FROM device_codes WHERE expires_at < NOW()
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup device codes: %w", err)
	}

	return result.RowsAffected(), nil
}

// Deny marks a device code as denied by the user.
func (s *DeviceCodeService) Deny(ctx context.Context, userCode string) error {
	// Simply delete the record to deny
	result, err := s.pool.Exec(ctx, `
		DELETE FROM device_codes
		WHERE user_code = $1 AND authorized_at IS NULL
	`, userCode)
	if err != nil {
		return fmt.Errorf("failed to deny device code: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrDeviceCodeNotFound
	}

	return nil
}
