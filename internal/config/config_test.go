package config

import (
	"strings"
	"testing"
	"time"
)

func validEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/glauk?sslmode=disable")
	t.Setenv("EVOLUTION_API_URL", "https://evolution.example")
	t.Setenv("EVOLUTION_INSTANCE", "Glauk")
	t.Setenv("GEMINI_API_KEY", "key-1")
	t.Setenv("R2_ACCESS_KEY_ID", "access")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	t.Setenv("R2_BUCKET_NAME", "bucket")
	t.Setenv("R2_ENDPOINT", "https://account.r2.cloudflarestorage.com")
}

func TestLoadDefaults(t *testing.T) {
	validEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SweeperInterval != time.Hour {
		t.Fatalf("intervalo = %v", cfg.SweeperInterval)
	}
	if cfg.ProcessedRetention != time.Hour || cfg.OrphanRetention != 24*time.Hour {
		t.Fatalf("retenciones = %v / %v", cfg.ProcessedRetention, cfg.OrphanRetention)
	}
	if cfg.DefaultInstance != "Glauk" {
		t.Fatalf("instancia = %q", cfg.DefaultInstance)
	}
}

func TestLoadRechazaURLInvalida(t *testing.T) {
	validEnv(t)
	t.Setenv("DATABASE_URL", "mysql://localhost/db")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("se esperaba error de DATABASE_URL, obtuve %v", err)
	}
}

func TestLoadRechazaSinClavesGemini(t *testing.T) {
	validEnv(t)
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GEMINI_KEYS", "")

	if _, err := Load(); err == nil {
		t.Fatal("se esperaba error por falta de claves Gemini")
	}
}
