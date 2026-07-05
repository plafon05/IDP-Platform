package auth

import (
	"testing"
	"time"

	"idp-platform/backend/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

func TestAccessTokenSupportsKeyRotation(t *testing.T) {
	oldConfig := tokenTestConfig([]config.JWTSigningKey{
		{KeyID: "old", Secret: "old-secret"},
	})
	oldToken, _, err := GenerateAccessToken(oldConfig, "user-1", []string{"employee"})
	if err != nil {
		t.Fatal(err)
	}

	rotatedConfig := tokenTestConfig([]config.JWTSigningKey{
		{KeyID: "current", Secret: "current-secret"},
		{KeyID: "old", Secret: "old-secret"},
	})
	claims, err := ParseAccessToken(rotatedConfig, oldToken)
	if err != nil {
		t.Fatalf("old token must remain valid during rotation: %v", err)
	}
	if claims.UserID != "user-1" {
		t.Fatalf("got user %q", claims.UserID)
	}

	newToken, _, err := GenerateAccessToken(rotatedConfig, "user-2", []string{"manager"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _, err := jwt.NewParser().ParseUnverified(newToken, &AccessClaims{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Header["kid"] != "current" {
		t.Fatalf("got kid %v, want current", parsed.Header["kid"])
	}
	if _, err := ParseAccessToken(rotatedConfig, newToken); err != nil {
		t.Fatalf("new token must validate with current key: %v", err)
	}
}

func TestAccessTokenRejectsUnknownKeyID(t *testing.T) {
	claims := AccessClaims{
		UserID: "user-1",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = "unknown"
	rawToken, err := token.SignedString([]byte("unknown-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAccessToken(tokenTestConfig([]config.JWTSigningKey{{KeyID: "current", Secret: "current-secret"}}), rawToken); err == nil {
		t.Fatal("token with unknown kid must be rejected")
	}
}

func TestAccessTokenRejectsMissingKeyID(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, AccessClaims{
		UserID:           "user-1",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	})
	rawToken, err := token.SignedString([]byte("current-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAccessToken(tokenTestConfig([]config.JWTSigningKey{{KeyID: "current", Secret: "current-secret"}}), rawToken); err == nil {
		t.Fatal("token without kid must be rejected")
	}
}

func TestAccessTokenUsesLegacySecretWithDefaultKeyID(t *testing.T) {
	cfg := config.Config{JWTSecret: "legacy-secret", JWTAccessTTL: time.Hour}
	rawToken, _, err := GenerateAccessToken(cfg, "user-1", []string{"employee"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _, err := jwt.NewParser().ParseUnverified(rawToken, &AccessClaims{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Header["kid"] != "default" {
		t.Fatalf("got kid %v, want default", parsed.Header["kid"])
	}
	if _, err := ParseAccessToken(cfg, rawToken); err != nil {
		t.Fatalf("legacy token must validate: %v", err)
	}
}

func tokenTestConfig(keys []config.JWTSigningKey) config.Config {
	return config.Config{JWTSecrets: keys, JWTAccessTTL: time.Hour}
}
