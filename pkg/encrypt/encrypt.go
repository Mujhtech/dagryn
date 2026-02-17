package encrypt

// Encrypt defines the interface for encryption/decryption operations.
type Encrypt interface {
	Encrypt(plaintext []byte) (string, error)
	Decrypt(ciphertext string) (string, error)
}
