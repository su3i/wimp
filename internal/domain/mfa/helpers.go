package mfa

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"time"
)

func ConstantTimeCompare(a, b uint32) bool {
	var r uint32
	r |= a ^ b
	return r == 0
}

func GenerateMFASecret() (string, error) {
	b := make([]byte, 20) // 160-bit
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base32.StdEncoding.
		WithPadding(base32.NoPadding).
		EncodeToString(b), nil
}

func GenerateTOTP(secret string, t time.Time) (uint32, error) {
	// Decode Base32 secret (Authenticator standard)
	key, err := base32.StdEncoding.
		WithPadding(base32.NoPadding).
		DecodeString(secret)
	if err != nil {
		return 0, err
	}

	// 30-second time step
	counter := uint64(t.Unix() / 30)

	// Convert counter to big-endian bytes
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	// HMAC-SHA1
	h := hmac.New(sha1.New, key)
	h.Write(buf[:])
	sum := h.Sum(nil)

	// Dynamic truncation
	offset := sum[len(sum)-1] & 0x0f
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff

	// 6-digit code
	return code % 1_000_000, nil
}