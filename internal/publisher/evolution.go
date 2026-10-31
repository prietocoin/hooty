package publisher

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxSendResponseBytes = 8 << 10

// EvolutionSender publica la cartelera en Evolution API.
type EvolutionSender struct {
	baseURL  string
	apiKey   string
	instance string
	http     *http.Client
}

// NewEvolutionSender valida la configuración y construye el cliente.
func NewEvolutionSender(baseURL, apiKey, instance string, timeout time.Duration) (*EvolutionSender, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("publisher: EVOLUTION_API_URL es obligatorio para publicar")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("publisher: EVOLUTION_API_URL inválida %q", baseURL)
	}
	if strings.TrimSpace(instance) == "" {
		return nil, fmt.Errorf("publisher: EVOLUTION_INSTANCE es obligatorio para publicar")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &EvolutionSender{
		baseURL:  baseURL,
		apiKey:   strings.TrimSpace(apiKey),
		instance: strings.TrimSpace(instance),
		http:     &http.Client{Timeout: timeout},
	}, nil
}

type sendMediaRequest struct {
	Number    string `json:"number"`
	MediaType string `json:"mediatype"`
	MimeType  string `json:"mimetype"`
	FileName  string `json:"fileName"`
	Media     string `json:"media"`
	Caption   string `json:"caption"`
}

// Send entrega la imagen por /message/sendMedia/{instance}. Un estado no 2xx se
// devuelve como error con el cuerpo recortado para diagnosticar 400/401/500.
func (s *EvolutionSender) Send(ctx context.Context, chatID string, png []byte, caption string) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return fmt.Errorf("publisher: chat_id vacío")
	}
	if len(png) == 0 {
		return fmt.Errorf("publisher: cartelera vacía")
	}
	body, err := json.Marshal(sendMediaRequest{
		Number:    chatID,
		MediaType: "image",
		MimeType:  "image/png",
		FileName:  "cartelera.png",
		Media:     base64.StdEncoding.EncodeToString(png),
		Caption:   caption,
	})
	if err != nil {
		return fmt.Errorf("publisher: serializar envío: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.baseURL+"/message/sendMedia/"+url.PathEscape(s.instance), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("publisher: construir envío: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("apikey", s.apiKey)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("publisher: enviar a Evolution: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxSendResponseBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("publisher: Evolution respondió HTTP %d: %s",
			resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}
