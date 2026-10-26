package ingest

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"hooty/internal/domain"
)

// HTTPHandler expone el webhook de Evolution.
type HTTPHandler struct {
	service         *Service
	token           string
	defaultInstance string
	maxBodyBytes    int64
	log             *slog.Logger
}

// HTTPOptions configura autenticación y límites del webhook.
type HTTPOptions struct {
	Token           string
	DefaultInstance string
	MaxBodyBytes    int64
	Logger          *slog.Logger
}

// NewHTTPHandler construye el handler del webhook.
func NewHTTPHandler(service *Service, opts HTTPOptions) *HTTPHandler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = 25 << 20
	}
	return &HTTPHandler{
		service:         service,
		token:           strings.TrimSpace(opts.Token),
		defaultInstance: strings.TrimSpace(opts.DefaultInstance),
		maxBodyBytes:    opts.MaxBodyBytes,
		log:             opts.Logger,
	}
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"status": "method_not_allowed"})
		return
	}
	if !h.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "unauthorized"})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, h.maxBodyBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "invalid_payload"})
		return
	}
	if int64(len(body)) > h.maxBodyBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"status": "payload_too_large"})
		return
	}

	event, err := ParseEvento(body, h.defaultInstance)
	if errors.Is(err, ErrNotImage) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	if err != nil {
		h.log.Warn("hooty.http.invalid_payload", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "invalid_payload"})
		return
	}

	decision, err := h.service.Recibir(r.Context(), event)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"status": "validation_error"})
			return
		}
		h.log.Error("hooty.http.ingest_failed",
			"hash", event.HashLargo,
			"grupo", event.IDGrupo,
			"error", err,
		)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error"})
		return
	}
	writeJSON(w, http.StatusOK, decision)
}

func (h *HTTPHandler) authorized(r *http.Request) bool {
	if h.token == "" {
		return true
	}
	provided := strings.TrimSpace(r.Header.Get("X-Hooty-Token"))
	if provided == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
			provided = strings.TrimSpace(auth[7:])
		}
	}
	if provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(h.token)) == 1
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
