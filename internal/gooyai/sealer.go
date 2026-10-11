package gooyai

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// ErrEncryptionNotConfigured means GOOYAI_ENCRYPTION_KEY is unset, so keys
// can be neither stored nor read.
var ErrEncryptionNotConfigured = errors.New("phone assistant key encryption is not configured")

// Sealer encrypts API keys at rest with AES-256-GCM.
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer builds a Sealer from a base64-encoded 32-byte key. An empty key
// yields a nil Sealer, so the service can start without the integration and
// report ErrEncryptionNotConfigured instead of failing boot.
func NewSealer(base64Key string) (*Sealer, error) {
	if base64Key == "" {
		return nil, nil //nolint:nilnil // nil Sealer = integration not configured
	}
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("GOOYAI_ENCRYPTION_KEY is not valid base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("GOOYAI_ENCRYPTION_KEY must decode to 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

// Seal returns nonce || ciphertext, authenticated with aad (additional data):
// what the value is and whose it is, e.g. phoneAssistantAAD("api_key", tenant).
// Open only succeeds with the same aad, so a ciphertext copied into another
// field or another clinic's settings fails to open instead of being read as
// that other thing.
func (s *Sealer) Seal(plaintext, aad string) ([]byte, error) {
	if s == nil {
		return nil, ErrEncryptionNotConfigured
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, []byte(plaintext), []byte(aad)), nil
}

// Open reverses Seal. It fails if the data was tampered with, sealed under
// another key (e.g. after rotating GOOYAI_ENCRYPTION_KEY) or sealed with
// another aad (moved to another field or tenant).
func (s *Sealer) Open(sealed []byte, aad string) (string, error) {
	if s == nil {
		return "", ErrEncryptionNotConfigured
	}
	n := s.aead.NonceSize()
	if len(sealed) < n {
		return "", errors.New("sealed gooyai key is too short")
	}
	plain, err := s.aead.Open(nil, sealed[:n], sealed[n:], []byte(aad))
	if err != nil {
		return "", fmt.Errorf("open sealed gooyai key: %w", err)
	}
	return string(plain), nil
}
