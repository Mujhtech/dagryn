// Package encrypt provides encryption utilities for securing sensitive data.
package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// Encrypt defines the interface for encryption/decryption operations.
type Encrypt interface {
	Encrypt(plaintext []byte) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// AESEncrypt implements the Encrypt interface using AES-GCM encryption.
type AESEncrypt struct {
	key []byte
}

// NewAESEncrypt creates a new AESEncrypt instance with the given key.
// The key must be 16, 24, or 32 bytes long for AES-128, AES-192, or AES-256.
func NewAESEncrypt(key string) (*AESEncrypt, error) {
	keyBytes := []byte(key)
	keyLen := len(keyBytes)

	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, errors.New("encryption key must be 16, 24, or 32 bytes long")
	}

	return &AESEncrypt{key: keyBytes}, nil
}

// Encrypt encrypts plaintext using AES-GCM and returns base64-encoded ciphertext.
func (e *AESEncrypt) Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext using AES-GCM.
func (e *AESEncrypt) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// NoOpEncrypt is a no-operation encrypter for development/testing.
// WARNING: This does not actually encrypt data!
type NoOpEncrypt struct{}

// NewNoOpEncrypt creates a new NoOpEncrypt instance.
func NewNoOpEncrypt() *NoOpEncrypt {
	return &NoOpEncrypt{}
}

// Encrypt returns the plaintext as base64 without encryption.
func (e *NoOpEncrypt) Encrypt(plaintext []byte) (string, error) {
	return base64.StdEncoding.EncodeToString(plaintext), nil
}

// Decrypt returns the base64-decoded plaintext.
func (e *NoOpEncrypt) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
