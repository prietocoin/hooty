// Package config carga la configuración de Hooty desde variables de entorno.
// No lee archivos .env: en desarrollo se exportan las variables o se usa el
// orquestador. Las credenciales nunca tienen valores por defecto.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config es la configuración completa del proceso.
type Config struct {
	HTTPAddr          string
	DatabaseURL       string
	WebhookToken      string
	DefaultInstance   string
	MaxBodyBytes      int64
	MaxImageBytes     int64
	ProcessingTimeout time.Duration
	ShutdownTimeout   time.Duration
	DBMaxConns        int32
	LogLevel          string
	LogFormat         string

	EvolutionURL     string
	EvolutionAPIKey  string
	EvolutionInst    string
	EvolutionTimeout time.Duration
	TestGroup        string

	GeminiKeys    []string
	GeminiModel   string
	GeminiTimeout time.Duration

	R2Endpoint     string
	R2Bucket       string
	R2AccessKey    string
	R2SecretKey    string
	R2PublicDomain string
	R2Region       string
	R2Timeout      time.Duration

	SweeperEnabled     bool
	SweeperInterval    time.Duration
	ProcessedRetention time.Duration
	OrphanRetention    time.Duration
}

// Load lee y valida el entorno.
func Load() (Config, error) {
	if err := validateRawEnv(); err != nil {
		return Config{}, err
	}
	cfg := Config{
		HTTPAddr:          env("HOOTY_ADDR", ":8080"),
		DatabaseURL:       strings.TrimSpace(os.Getenv("DATABASE_URL")),
		WebhookToken:      strings.TrimSpace(os.Getenv("HOOTY_WEBHOOK_TOKEN")),
		DefaultInstance:   env("HOOTY_DEFAULT_INSTANCE", env("EVOLUTION_INSTANCE", "hooty")),
		MaxBodyBytes:      envInt64("HOOTY_MAX_BODY_BYTES", 25<<20),
		MaxImageBytes:     envInt64("HOOTY_MAX_IMAGE_BYTES", 20<<20),
		ProcessingTimeout: envDuration("HOOTY_PROCESSING_TIMEOUT", 2*time.Minute),
		ShutdownTimeout:   envDuration("HOOTY_SHUTDOWN_TIMEOUT", 15*time.Second),
		DBMaxConns:        int32(envInt64("HOOTY_DB_MAX_CONNS", 10)),
		LogLevel:          strings.ToLower(env("HOOTY_LOG_LEVEL", "info")),
		LogFormat:         strings.ToLower(env("HOOTY_LOG_FORMAT", "text")),

		EvolutionURL:     env("EVOLUTION_API_URL", ""),
		EvolutionAPIKey:  strings.TrimSpace(os.Getenv("EVOLUTION_API_KEY")),
		EvolutionInst:    env("EVOLUTION_INSTANCE", "hooty"),
		EvolutionTimeout: envDuration("EVOLUTION_TIMEOUT", 30*time.Second),
		TestGroup:        env("EVOLUTION_TEST_GROUP", env("HOOTY_TEST_GROUP", "")),

		GeminiKeys:    splitCSV(os.Getenv("GEMINI_KEYS")),
		GeminiModel:   env("GEMINI_MODEL", "gemini-2.0-flash"),
		GeminiTimeout: envDuration("GEMINI_TIMEOUT", 45*time.Second),

		R2Endpoint:     strings.TrimRight(env("R2_ENDPOINT", ""), "/"),
		R2Bucket:       env("R2_BUCKET_NAME", "remesas-img"),
		R2AccessKey:    strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		R2SecretKey:    strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY")),
		R2PublicDomain: strings.TrimRight(env("R2_PUBLIC_DOMAIN", ""), "/"),
		R2Region:       env("R2_REGION", "auto"),
		R2Timeout:      envDuration("R2_TIMEOUT", 60*time.Second),

		SweeperEnabled:     envBool("HOOTY_SWEEPER_ENABLED", true),
		SweeperInterval:    envDuration("HOOTY_SWEEPER_INTERVAL", time.Hour),
		ProcessedRetention: envDuration("HOOTY_PROCESSED_RETENTION", time.Hour),
		OrphanRetention:    envDuration("HOOTY_ORPHAN_RETENTION", 24*time.Hour),
	}
	if len(cfg.GeminiKeys) == 0 {
		cfg.GeminiKeys = splitCSV(os.Getenv("GEMINI_API_KEY"))
	}
	if cfg.R2Endpoint == "" {
		if account := strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID")); account != "" {
			cfg.R2Endpoint = "https://" + account + ".r2.cloudflarestorage.com"
		}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate falla antes de abrir conexiones o aceptar webhooks.
func (c Config) Validate() error {
	if err := validateURL(c.DatabaseURL, "postgres", "postgresql"); err != nil {
		return fmt.Errorf("config: DATABASE_URL: %w", err)
	}
	if strings.TrimSpace(c.HTTPAddr) == "" {
		return errors.New("config: HOOTY_ADDR es obligatorio")
	}
	if c.MaxBodyBytes <= 0 {
		return errors.New("config: HOOTY_MAX_BODY_BYTES debe ser positivo")
	}
	if c.MaxImageBytes <= 0 {
		return errors.New("config: HOOTY_MAX_IMAGE_BYTES debe ser positivo")
	}
	if c.ProcessingTimeout <= 0 {
		return errors.New("config: HOOTY_PROCESSING_TIMEOUT debe ser positivo")
	}
	if c.ShutdownTimeout <= 0 {
		return errors.New("config: HOOTY_SHUTDOWN_TIMEOUT debe ser positivo")
	}
	if c.DBMaxConns <= 0 {
		return errors.New("config: HOOTY_DB_MAX_CONNS debe ser positivo")
	}
	if err := validateURL(c.EvolutionURL, "http", "https"); err != nil {
		return fmt.Errorf("config: EVOLUTION_API_URL: %w", err)
	}
	if strings.TrimSpace(c.EvolutionInst) == "" {
		return errors.New("config: EVOLUTION_INSTANCE es obligatorio")
	}
	if len(c.GeminiKeys) == 0 {
		return errors.New("config: GEMINI_API_KEY o GEMINI_KEYS es obligatorio")
	}
	if err := validateURL(c.R2Endpoint, "http", "https"); err != nil {
		return fmt.Errorf("config: R2_ENDPOINT/R2_ACCOUNT_ID: %w", err)
	}
	if strings.TrimSpace(c.R2Bucket) == "" || strings.TrimSpace(c.R2AccessKey) == "" ||
		strings.TrimSpace(c.R2SecretKey) == "" {
		return errors.New("config: R2_BUCKET_NAME, R2_ACCESS_KEY_ID y R2_SECRET_ACCESS_KEY son obligatorios")
	}
	if c.SweeperEnabled {
		if c.SweeperInterval <= 0 || c.ProcessedRetention <= 0 || c.OrphanRetention <= 0 {
			return errors.New("config: las retenciones y el intervalo del sweeper deben ser positivos")
		}
	}
	switch c.LogFormat {
	case "text", "json":
	default:
		return fmt.Errorf("config: HOOTY_LOG_FORMAT inválido %q", c.LogFormat)
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("config: HOOTY_LOG_LEVEL inválido %q", c.LogLevel)
	}
	return nil
}

func validateRawEnv() error {
	for _, key := range []string{
		"HOOTY_PROCESSING_TIMEOUT", "HOOTY_SHUTDOWN_TIMEOUT",
		"EVOLUTION_TIMEOUT", "GEMINI_TIMEOUT", "R2_TIMEOUT",
		"HOOTY_SWEEPER_INTERVAL", "HOOTY_PROCESSED_RETENTION", "HOOTY_ORPHAN_RETENTION",
	} {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}
		if _, err := time.ParseDuration(raw); err != nil {
			return fmt.Errorf("config: %s inválido: %w", key, err)
		}
	}
	for _, key := range []string{"HOOTY_MAX_BODY_BYTES", "HOOTY_MAX_IMAGE_BYTES", "HOOTY_DB_MAX_CONNS"} {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}
		if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
			return fmt.Errorf("config: %s inválido: %w", key, err)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("HOOTY_SWEEPER_ENABLED")); raw != "" {
		if _, err := strconv.ParseBool(raw); err != nil {
			return fmt.Errorf("config: HOOTY_SWEEPER_ENABLED inválido: %w", err)
		}
	}
	return nil
}

func validateURL(raw string, schemes ...string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if parsed.Host == "" {
		return errors.New("URL sin host")
	}
	for _, scheme := range schemes {
		if parsed.Scheme == scheme {
			return nil
		}
	}
	return fmt.Errorf("esquema %q no permitido", parsed.Scheme)
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	var values []string
	seen := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		clean := strings.TrimSpace(item)
		if clean == "" || seen[clean] {
			continue
		}
		seen[clean] = true
		values = append(values, clean)
	}
	return values
}
