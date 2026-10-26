// Package evolution descarga el binario de un mensaje de WhatsApp desde la API
// de Evolution. No conoce PostgreSQL ni el pipeline: recibe la key/message
// crudos y devuelve bytes.
package evolution

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrMediaUnavailable indica que la instancia no devolvió el binario.
var ErrMediaUnavailable = errors.New("evolution: media no disponible")

const maxMediaResponseBytes = 32 << 20

// Config reúne las credenciales mínimas de Evolution.
type Config struct {
	BaseURL  string
	APIKey   string
	Instance string
	Timeout  time.Duration
}

// Client implementa ingest.MediaFetcher.
type Client struct {
	baseURL  string
	apiKey   string
	instance string
	http     *http.Client
}

// New valida la configuración y construye el cliente.
func New(cfg Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("evolution: EVOLUTION_API_URL es obligatorio")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("evolution: URL inválida %q", cfg.BaseURL)
	}
	if strings.TrimSpace(cfg.Instance) == "" {
		return nil, errors.New("evolution: EVOLUTION_INSTANCE es obligatorio")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &Client{
		baseURL:  baseURL,
		apiKey:   strings.TrimSpace(cfg.APIKey),
		instance: strings.TrimSpace(cfg.Instance),
		http:     &http.Client{Timeout: cfg.Timeout},
	}, nil
}

type mediaResponse struct {
	Base64   string `json:"base64"`
	Media    string `json:"media"`
	Mimetype string `json:"mimetype"`
}

// Descargar pide el binario del mensaje. Se prueban las dos rutas conocidas
// porque distintas versiones de Evolution exponen prefijos diferentes.
func (c *Client) Descargar(ctx context.Context, instance string, key, message json.RawMessage) ([]byte, string, error) {
	target := firstNonEmpty(instance, c.instance)
	if target == "" {
		return nil, "", errors.New("evolution: instancia vacía")
	}
	if len(key) == 0 || len(message) == 0 {
		return nil, "", errors.New("evolution: key y message son obligatorios")
	}
	payload, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"key":     json.RawMessage(key),
			"message": json.RawMessage(message),
		},
		"convertToMp4": false,
	})
	if err != nil {
		return nil, "", fmt.Errorf("evolution: serializar descarga: %w", err)
	}

	paths := []string{
		"/message/getBase64FromMediaMessage/" + url.PathEscape(target),
		"/chat/getBase64FromMediaMessage/" + url.PathEscape(target),
	}
	var lastErr error
	for _, path := range paths {
		data, mime, retry, err := c.once(ctx, path, payload)
		if err == nil {
			return data, mime, nil
		}
		lastErr = err
		if !retry {
			return nil, "", err
		}
	}
	if lastErr == nil {
		lastErr = ErrMediaUnavailable
	}
	return nil, "", lastErr
}

func (c *Client) once(ctx context.Context, path string, body []byte) ([]byte, string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, "", false, fmt.Errorf("evolution: construir descarga: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("apikey", c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, "", false, ctxErr
		}
		return nil, "", true, fmt.Errorf("evolution: descargar media: %w", err)
	}
	defer resp.Body.Close()

	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxMediaResponseBytes))
	if readErr != nil {
		return nil, "", true, fmt.Errorf("evolution: leer media: %w", readErr)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", true, ErrMediaUnavailable
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return nil, "", retry, fmt.Errorf("evolution: estado %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}

	var parsed mediaResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, "", false, fmt.Errorf("evolution: decodificar media: %w", err)
	}
	encoded := firstNonEmpty(parsed.Base64, parsed.Media)
	if encoded == "" {
		return nil, "", false, ErrMediaUnavailable
	}
	if index := strings.Index(encoded, "base64,"); index >= 0 {
		encoded = encoded[index+len("base64,"):]
	}
	decoded, err := decodeBase64(encoded)
	if err != nil {
		return nil, "", false, fmt.Errorf("evolution: decodificar base64: %w", err)
	}
	return decoded, firstNonEmpty(parsed.Mimetype, "image/jpeg"), false, nil
}

func decodeBase64(value string) ([]byte, error) {
	clean := strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ':
			return -1
		}
		return r
	}, value)
	if data, err := base64.StdEncoding.DecodeString(clean); err == nil {
		return data, nil
	}
	return base64.RawStdEncoding.DecodeString(clean)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func truncate(value string, limit int) string {
	clean := strings.TrimSpace(value)
	if len(clean) <= limit {
		return clean
	}
	return clean[:limit] + "..."
}
