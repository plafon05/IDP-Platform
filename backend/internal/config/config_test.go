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

func TestValidateProductionRejectsUnsafeDefaults(t *testing.T) {
	cfg := Config{AppEnv: "production", JWTSecrets: []JWTSigningKey{{KeyID: "default", Secret: "local-development-secret-change-me"}}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected unsafe production configuration to be rejected")
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
