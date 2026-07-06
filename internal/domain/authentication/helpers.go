package authentication

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateJWT(p JWTParams) (string, error) {
	now := time.Now()

	claims := jwt.MapClaims{
		"sub":   p.Subject,
		"username": p.Username,
		"roles": p.Roles,
		"iss":   "https://wimp.suei.io/",
		"iat":   now.Unix(),
		"exp":   now.Add(p.TTL).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(p.SecretKey)
}

func GenerateRefreshToken() (raw string, hash string, err error) {
	b := make([]byte, 32)

	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}

	raw = base64.RawURLEncoding.EncodeToString(b)
	tmp := sha256.Sum256([]byte(raw))

	return raw, hex.EncodeToString(tmp[:]), nil
}

func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}