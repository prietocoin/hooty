package publisher

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	log      *slog.Logger
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
		log:      slog.Default(),
	}, nil
}

// WithLogger conecta el logger del proceso para registrar URL, método y
// cuerpo de respuesta cuando Evolution rechaza un envío.
func (s *EvolutionSender) WithLogger(logger *slog.Logger) *EvolutionSender {
	if logger != nil {
		s.log = logger
	}
	return s
}

type sendMediaRequest struct {
	Number    string `json:"number"`
	MediaType string `json:"mediatype"`
	MimeType  string `json:"mimetype"`
	FileName  string `json:"fileName"`
	Media     string `json:"media"`
	Caption   string `json:"caption"`
}

// Send entrega la cartelera PNG por /message/sendMedia/{instance}.
func (s *EvolutionSender) Send(ctx context.Context, chatID string, png []byte, caption string) error {
	return s.SendMedia(ctx, chatID, png, "image/png", "cartelera.png", caption)
}

// SendMedia entrega cualquier archivo compatible con Evolution API v2 usando
// estrictamente POST /message/sendMedia/{instance}. Un estado no 2xx se
// devuelve como error y se registra con URL, método y cuerpo de respuesta.
func (s *EvolutionSender) SendMedia(ctx context.Context, chatID string, data []byte,
	mimeType, fileName, caption string) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return fmt.Errorf("publisher: chat_id vacío")
	}
	if len(data) == 0 {
		return fmt.Errorf("publisher: media vacía")
	}
	mimeType = strings.TrimSpace(mimeType)
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = nombreArchivoMedia(mimeType)
	}
	mediaType := "document"
	if strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		mediaType = "image"
	}

	body, err := json.Marshal(sendMediaRequest{
		Number:    chatID,
		MediaType: mediaType,
		MimeType:  mimeType,
		FileName:  fileName,
		Media:     base64.StdEncoding.EncodeToString(data),
		Caption:   caption,
	})
	if err != nil {
		return fmt.Errorf("publisher: serializar envío: %w", err)
	}

	endpoint := s.baseURL + "/message/sendMedia/" + url.PathEscape(s.instance)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("publisher: construir envío: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("apikey", s.apiKey)
	}

	s.log.Info("hooty.evolution.send_media",
		"url", endpoint, "method", http.MethodPost, "instance", s.instance,
		"chat_id", chatID, "media_type", mediaType, "mime", mimeType,
		"file_name", fileName, "bytes", len(data))

	resp, err := s.http.Do(req)
	if err != nil {
		s.log.Error("hooty.evolution.send_media_error",
			"url", endpoint, "method", http.MethodPost, "chat_id", chatID, "error", err)
		return fmt.Errorf("publisher: enviar a Evolution: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxSendResponseBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respuesta := strings.TrimSpace(string(raw))
		s.log.Error("hooty.evolution.send_media_status",
			"url", endpoint, "method", http.MethodPost, "status", resp.StatusCode,
			"chat_id", chatID, "body", recortarRespuesta(respuesta))
		return fmt.Errorf("publisher: Evolution respondió HTTP %d: %s",
			resp.StatusCode, respuesta)
	}
	s.log.Info("hooty.evolution.send_media_ok",
		"url", endpoint, "method", http.MethodPost, "status", resp.StatusCode, "chat_id", chatID)
	return nil
}

func nombreArchivoMedia(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg":
		return "reporte.jpg"
	case "image/webp":
		return "reporte.webp"
	case "application/pdf":
		return "reporte.pdf"
	default:
		return "reporte.png"
	}
}

func recortarRespuesta(value string) string {
	const maximo = 2000
	if len(value) <= maximo {
		return value
	}
	return value[:maximo] + "…"
}
