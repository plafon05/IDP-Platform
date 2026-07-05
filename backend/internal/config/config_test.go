package config

import "testing"

func TestLoadJWTSecrets(t *testing.T) {
	t.Setenv("JWT_SECRETS", "current:new-secret,old:old-secret")
	t.Setenv("JWT_SECRET", "legacy-secret")
	keys := Load().JWTSecrets
	if len(keys) != 2 || keys[0].KeyID != "current" || keys[0].Secret != "new-secret" || keys[1].KeyID != "old" {
		t.Fatalf("unexpected keys: %#v", keys)
	}
}

func TestLoadJWTSecretLegacyFallback(t *testing.T) {
	t.Setenv("JWT_SECRETS", "")
	t.Setenv("JWT_SECRET", "legacy-secret")
	keys := Load().JWTSecrets
	if len(keys) != 1 || keys[0].KeyID != "default" || keys[0].Secret != "legacy-secret" {
		t.Fatalf("unexpected legacy keys: %#v", keys)
	}
}
