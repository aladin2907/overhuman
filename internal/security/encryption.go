package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// Credential encryption — AES-256-GCM
// ---------------------------------------------------------------------------

// Encryptor handles encryption/decryption of sensitive values using
// AES-256-GCM (authenticated encryption). The master key is derived
// via SHA-256 from a passphrase.
type Encryptor struct {
	mu     sync.RWMutex
	aead   cipher.AEAD
	prefix string // Encrypted values are prefixed with this to identify them.
}

const encryptedPrefix = "enc:v1:"

// NewEncryptor creates an Encryptor from a master key passphrase.
// The passphrase is hashed with SHA-256 to produce a 32-byte AES key.
func NewEncryptor(passphrase string) (*Encryptor, error) {
	if len(passphrase) < 8 {
		return nil, fmt.Errorf("passphrase must be at least 8 characters")
	}

	// Derive a 32-byte key using SHA-256.
	hash := sha256.Sum256([]byte(passphrase))

	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	return &Encryptor{
		aead:   aead,
		prefix: encryptedPrefix,
	}, nil
}

// Encrypt encrypts a plaintext value and returns a base64-encoded ciphertext
// prefixed with "enc:v1:" to identify it as encrypted.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := e.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	return e.prefix + encoded, nil
}

// Decrypt decrypts a value encrypted by Encrypt(). If the value doesn't
// have the encryption prefix, it's returned as-is (for backward compat).
func (e *Encryptor) Decrypt(encrypted string) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !strings.HasPrefix(encrypted, e.prefix) {
		// Not encrypted — return as-is for backward compatibility.
		return encrypted, nil
	}

	encoded := encrypted[len(e.prefix):]
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	nonceSize := e.aead.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	plaintext, err := e.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// IsEncrypted checks if a value has the encryption prefix.
func (e *Encryptor) IsEncrypted(value string) bool {
	return strings.HasPrefix(value, e.prefix)
}

// ---------------------------------------------------------------------------
// Value masking — for logs and output
// ---------------------------------------------------------------------------

// MaskSecret masks a secret value for display in logs/output.
// Shows first N and last N characters, middle replaced with asterisks.
func MaskSecret(value string, showChars int) string {
	if len(value) <= showChars*2 {
		return strings.Repeat("*", len(value))
	}
	return value[:showChars] + strings.Repeat("*", len(value)-showChars*2) + value[len(value)-showChars:]
}

// MaskInString replaces occurrences of a secret within a string.
// Used to prevent credential leakage in logs and outputs.
func MaskInString(text, secret string) string {
	if secret == "" || len(secret) < 4 {
		return text
	}
	return strings.ReplaceAll(text, secret, MaskSecret(secret, 2))
}

// SecretRegistry tracks known secret values for output masking.
type SecretRegistry struct {
	mu      sync.RWMutex
	secrets []string
}

// NewSecretRegistry creates an empty secret registry.
func NewSecretRegistry() *SecretRegistry {
	return &SecretRegistry{
		secrets: make([]string, 0),
	}
}

// Register adds a secret value that should be masked in outputs.
func (sr *SecretRegistry) Register(secret string) {
	if secret == "" || len(secret) < 4 {
		return
	}
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.secrets = append(sr.secrets, secret)
}

// Remove removes a secret from the registry.
func (sr *SecretRegistry) Remove(secret string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	for i, s := range sr.secrets {
		if s == secret {
			sr.secrets = append(sr.secrets[:i], sr.secrets[i+1:]...)
			return
		}
	}
}

// Sanitize replaces all known secrets in the text with masked versions.
func (sr *SecretRegistry) Sanitize(text string) string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	for _, secret := range sr.secrets {
		text = MaskInString(text, secret)
	}
	return text
}

// Count returns the number of registered secrets.
func (sr *SecretRegistry) Count() int {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return len(sr.secrets)
}
