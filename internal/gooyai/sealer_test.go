package gooyai

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSealer(t *testing.T) {
	s, err := NewSealer("")
	require.NoError(t, err)
	assert.Nil(t, s, "unset key = integration disabled, not a boot failure")

	_, err = NewSealer("%%%not-base64")
	assert.ErrorContains(t, err, "not valid base64")

	_, err = NewSealer(base64.StdEncoding.EncodeToString([]byte("too-short")))
	assert.ErrorContains(t, err, "must decode to 32 bytes, got 9")
}

func TestSealer_RoundTrip(t *testing.T) {
	s := testSealer(t)

	a, err := s.Seal("sk-live-secret")
	require.NoError(t, err)
	b, err := s.Seal("sk-live-secret")
	require.NoError(t, err)

	assert.NotEqual(t, a, b, "a fresh nonce per seal")
	assert.NotContains(t, string(a), "sk-live-secret")

	plain, err := s.Open(a)
	require.NoError(t, err)
	assert.Equal(t, "sk-live-secret", plain)
}

func TestSealer_OpenRejectsTamperingAndOtherKeys(t *testing.T) {
	s := testSealer(t)
	sealed, err := s.Seal("sk-live-secret")
	require.NoError(t, err)

	tampered := append([]byte{}, sealed...)
	tampered[len(tampered)-1] ^= 0xff
	_, err = s.Open(tampered)
	assert.ErrorContains(t, err, "open sealed gooyai key")

	other, err := NewSealer(base64.StdEncoding.EncodeToString([]byte("ffffffffffffffffffffffffffffffff")))
	require.NoError(t, err)
	_, err = other.Open(sealed)
	assert.ErrorContains(t, err, "open sealed gooyai key")

	_, err = s.Open([]byte("short"))
	assert.ErrorContains(t, err, "too short")
}

func TestSealer_NilMeansNotConfigured(t *testing.T) {
	var s *Sealer
	_, err := s.Seal("x")
	assert.ErrorIs(t, err, ErrEncryptionNotConfigured)
	_, err = s.Open([]byte("x"))
	assert.ErrorIs(t, err, ErrEncryptionNotConfigured)
}
