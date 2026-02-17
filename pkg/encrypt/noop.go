package encrypt

import "encoding/base64"

// NoOpEncrypt is a no-operation encrypter for development/testing.
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
