package authz

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mujhtech/dagryn/pkg/database/models"
	"github.com/mujhtech/dagryn/pkg/database/repo"
)

var (
	// ErrInvalidToken is returned when a token is invalid.
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken is returned when a token has expired.
	ErrExpiredToken = errors.New("token expired")
	// ErrRevokedToken is returned when a token has been revoked.
	ErrRevokedToken = errors.New("token revoked")
	// ErrInvalidClaims is returned when token claims are invalid.
	ErrInvalidClaims = errors.New("invalid token claims")
)

// JWTConfig holds JWT configuration.
type JWTConfig struct {
	Secret        string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
	Issuer        string
}

// Claims represents the JWT claims.
type Claims struct {
	jwt.RegisteredClaims
	UserID uuid.UUID `json:"uid"`
	Email  string    `json:"email"`
	Type   string    `json:"type"` // "access" or "refresh"
}

// TokenPair represents an access and refresh token pair.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// JWTService handles JWT token operations.
type JWTService struct {
	config     JWTConfig
	tokenRepo  *repo.TokenRepo
	signingKey []byte
}

// NewJWTService creates a new JWT service.
func NewJWTService(config JWTConfig, tokenRepo *repo.TokenRepo) *JWTService {
	return &JWTService{
		config:     config,
		tokenRepo:  tokenRepo,
		signingKey: []byte(config.Secret),
	}
}

// GenerateTokenPair generates a new access and refresh token pair.
func (s *JWTService) GenerateTokenPair(ctx context.Context, user *models.User) (*TokenPair, error) {
	now := time.Now()

	// Generate access token
	accessJTI := uuid.New().String()
	accessExpiry := now.Add(s.config.AccessExpiry)
	accessToken, err := s.generateToken(user, accessJTI, "access", accessExpiry)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshJTI := uuid.New().String()
	refreshExpiry := now.Add(s.config.RefreshExpiry)
	refreshToken, err := s.generateToken(user, refreshJTI, "refresh", refreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Store tokens for tracking/revocation
	if err := s.tokenRepo.Create(ctx, &models.Token{
		UserID:    user.ID,
		TokenType: models.TokenTypeAccess,
		JTI:       accessJTI,
		IssuedAt:  now,
		ExpiresAt: accessExpiry,
	}); err != nil {
		return nil, fmt.Errorf("failed to store access token: %w", err)
	}

	if err := s.tokenRepo.Create(ctx, &models.Token{
		UserID:    user.ID,
		TokenType: models.TokenTypeRefresh,
		JTI:       refreshJTI,
		IssuedAt:  now,
		ExpiresAt: refreshExpiry,
	}); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.config.AccessExpiry.Seconds()),
		ExpiresAt:    accessExpiry,
	}, nil
}

// generateToken generates a single JWT token.
func (s *JWTService) generateToken(user *models.User, jti string, tokenType string, expiry time.Time) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.config.Issuer,
			Subject:   user.ID.String(),
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        jti,
		},
		UserID: user.ID,
		Email:  user.Email,
		Type:   tokenType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.signingKey)
}

// ValidateAccessToken validates an access token and returns the claims.
func (s *JWTService) ValidateAccessToken(ctx context.Context, tokenString string) (*Claims, error) {
	claims, err := s.parseToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.Type != "access" {
		return nil, ErrInvalidToken
	}

	// Check if token is revoked by looking up in database
	_, err = s.tokenRepo.GetValidByJTI(ctx, claims.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrRevokedToken
		}
		return nil, fmt.Errorf("failed to check token: %w", err)
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token and returns the claims.
func (s *JWTService) ValidateRefreshToken(ctx context.Context, tokenString string) (*Claims, error) {
	claims, err := s.parseToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.Type != "refresh" {
		return nil, ErrInvalidToken
	}

	// Check if token is revoked by looking up in database
	_, err = s.tokenRepo.GetValidByJTI(ctx, claims.ID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrRevokedToken
		}
		return nil, fmt.Errorf("failed to check token: %w", err)
	}

	return claims, nil
}

// parseToken parses and validates a JWT token.
func (s *JWTService) parseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.signingKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}

// RefreshTokens generates a new token pair using a valid refresh token.
func (s *JWTService) RefreshTokens(ctx context.Context, refreshToken string, user *models.User) (*TokenPair, error) {
	claims, err := s.ValidateRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	// Revoke old refresh token
	if err := s.tokenRepo.Revoke(ctx, claims.ID); err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, fmt.Errorf("failed to revoke old refresh token: %w", err)
	}

	// Generate new token pair
	return s.GenerateTokenPair(ctx, user)
}

// RevokeToken revokes a token by its JTI.
func (s *JWTService) RevokeToken(ctx context.Context, jti string) error {
	return s.tokenRepo.Revoke(ctx, jti)
}

// RevokeAllUserTokens revokes all tokens for a user.
func (s *JWTService) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	return s.tokenRepo.RevokeAllForUser(ctx, userID)
}

// UpdateLastUsed updates the last used timestamp for a token.
func (s *JWTService) UpdateLastUsed(ctx context.Context, jti string) error {
	return s.tokenRepo.UpdateLastUsed(ctx, jti)
}

// GenerateRandomState generates a random state string for OAuth.
func GenerateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GenerateDeviceCode generates a device code for CLI authentication.
func GenerateDeviceCode() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GenerateUserCode generates a user-friendly code for device flow.
func GenerateUserCode() (string, error) {
	// Generate 8 character alphanumeric code (e.g., "ABCD-1234")
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Removed confusing chars (0, O, 1, I)
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	// Format as XXXX-XXXX
	return string(b[:4]) + "-" + string(b[4:]), nil
}
