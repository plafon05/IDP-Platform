package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"idp-platform/backend/internal/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccessClaims struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(cfg config.Config, userID string, roles []string) (string, time.Time, error) {
	keys := cfg.AccessTokenKeys()
	if len(keys) == 0 {
		return "", time.Time{}, errors.New("access token signing key is not configured")
	}
	if keys[0].KeyID == "" || keys[0].Secret == "" {
		return "", time.Time{}, errors.New("access token signing key is invalid")
	}
	expiresAt := time.Now().Add(cfg.JWTAccessTTL)
	claims := AccessClaims{
		UserID: userID,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = keys[0].KeyID
	signed, err := token.SignedString([]byte(keys[0].Secret))
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiresAt, nil
}

func ParseAccessToken(cfg config.Config, rawToken string) (*AccessClaims, error) {
	keys := cfg.AccessTokenKeys()
	token, err := jwt.ParseWithClaims(rawToken, &AccessClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		keyID, ok := token.Header["kid"].(string)
		if !ok || keyID == "" {
			return nil, errors.New("access token kid is missing")
		}
		for _, key := range keys {
			if key.KeyID == keyID {
				return []byte(key.Secret), nil
			}
		}
		return nil, fmt.Errorf("unknown access token kid: %s", keyID)
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid access token")
	}

	return claims, nil
}

func GenerateRefreshToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}

	token := base64.RawURLEncoding.EncodeToString(bytes)
	return token, HashRefreshToken(token), nil
}

func GeneratePasswordResetToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}

	token := base64.RawURLEncoding.EncodeToString(bytes)
	return token, HashPasswordResetToken(token), nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func HashPasswordResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func StoreRefreshToken(ctx context.Context, db *pgxpool.Pool, userID string, tokenHash string, expiresAt time.Time) error {
	_, err := db.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	return err
}

func RevokeRefreshToken(ctx context.Context, db *pgxpool.Pool, tokenHash string) error {
	_, err := db.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, tokenHash)
	return err
}
