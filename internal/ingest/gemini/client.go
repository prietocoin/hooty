// Package gemini implementa el extractor de comprobantes con la API REST de
// Google. Devuelve solo los cinco campos del contrato; la validación de negocio
// queda en domain.Extraccion.Validate.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"hooty/internal/domain"
)

const baseURL = "https://generativelanguage.googleapis.com/v1beta/models/"
const maxResponseBytes = 4 << 20

var fallbackModels = []string{
	"gemini-2.0-flash",
	"gemini-2.0-flash-lite",
	"gemini-1.5-flash",
	"gemini-1.5-flash-8b",
}

// Config configura el extractor.
type Config struct {
	APIKeys []string
	Model   string
	Timeout time.Duration
	Client  *http.Client
}

// Client rota claves y modelos ante 429/503.
type Client struct {
	keys   []string
	models []string
	http   *http.Client
	next   atomic.Uint64
}

// New valida las claves y construye el cliente.
func New(cfg Config) (*Client, error) {
	keys := uniqueNonEmpty(cfg.APIKeys)
	if len(keys) == 0 {
		return nil, fmt.Errorf("gemini: no hay claves configuradas")
	}
	models := uniqueNonEmpty(append([]string{cfg.Model}, fallbackModels...))
	if cfg.Timeout <= 0 {
		cfg.Timeout = 45 * time.Second
	}
	httpClient := cfg.Client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: cfg.Timeout}
	}
	return &Client{keys: keys, models: models, http: httpClient}, nil
}

// Configured indica si el cliente puede extraer.
func (c *Client) Configured() bool {
	return c != nil && len(c.keys) > 0
}

// Extraer envía la imagen y el caption y devuelve los campos normalizados.
func (c *Client) Extraer(ctx context.Context, image []byte, mimeType, caption string) (domain.Extraccion, error) {
	if !c.Configured() {
		return domain.Extraccion{}, fmt.Errorf("gemini: no hay claves configuradas")
	}
	if len(image) == 0 {
		return domain.Extraccion{}, fmt.Errorf("gemini: la imagen está vacía")
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = http.DetectContentType(image)
	}

	body, err := json.Marshal(request{
		Contents: []content{{
			Parts: []part{
				{Text: PromptWithCaption(DefaultPrompt, caption)},
				{InlineData: &inlineData{MimeType: mimeType, Data: image}},
			},
		}},
		GenerationConfig: generation{MimeType: "application/json", Temperature: 0.1},
	})
	if err != nil {
		return domain.Extraccion{}, fmt.Errorf("gemini: serializar petición: %w", err)
	}

	var lastErr error
	for _, model := range c.models {
		for attempt := 0; attempt < 3; attempt++ {
			if err := ctx.Err(); err != nil {
				return domain.Extraccion{}, err
			}
			key := c.takeKey()
			result, retry, err := c.once(ctx, model, key, body)
			if err == nil {
				return result, nil
			}
			lastErr = err
			if !retry {
				break
			}
			if err := sleep(ctx, time.Duration(150*(attempt+1))*time.Millisecond); err != nil {
				return domain.Extraccion{}, err
			}
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("gemini: sin respuesta")
	}
	return domain.Extraccion{}, fmt.Errorf("gemini: fallaron todos los modelos: %w", lastErr)
}

func (c *Client) takeKey() string {
	index := c.next.Add(1) - 1
	return c.keys[index%uint64(len(c.keys))]
}

func (c *Client) once(ctx context.Context, model, key string, body []byte) (domain.Extraccion, bool, error) {
	endpoint := baseURL + url.PathEscape(model) + ":generateContent?key=" + url.QueryEscape(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return domain.Extraccion{}, false, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return domain.Extraccion{}, false, ctxErr
		}
		return domain.Extraccion{}, true, err
	}
	defer resp.Body.Close()

	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if readErr != nil {
		return domain.Extraccion{}, true, fmt.Errorf("leer respuesta: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.Extraccion{}, retryable(resp.StatusCode),
			fmt.Errorf("estado %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var parsed response
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return domain.Extraccion{}, true, fmt.Errorf("decodificar respuesta: %w", err)
	}
	text := parsed.text()
	if text == "" {
		return domain.Extraccion{}, true, fmt.Errorf("respuesta sin contenido")
	}
	result, err := parseJSON(text)
	if err != nil {
		return domain.Extraccion{}, true, err
	}
	return result, false, nil
}

func retryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

func truncate(value string, limit int) string {
	clean := strings.TrimSpace(value)
	if len(clean) <= limit {
		return clean
	}
	return clean[:limit] + "..."
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

func sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type request struct {
	Contents         []content  `json:"contents"`
	GenerationConfig generation `json:"generationConfig"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inline_data,omitempty"`
}

type inlineData struct {
	MimeType string `json:"mime_type"`
	Data     []byte `json:"data"`
}

type generation struct {
	MimeType    string  `json:"responseMimeType"`
	Temperature float64 `json:"temperature"`
}

type response struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (r response) text() string {
	var builder strings.Builder
	for _, candidate := range r.Candidates {
		for _, p := range candidate.Content.Parts {
			builder.WriteString(p.Text)
		}
	}
	return strings.TrimSpace(builder.String())
}
