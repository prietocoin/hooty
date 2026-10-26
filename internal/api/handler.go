package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"hooty/internal/domain"
)

// Store es la superficie de lectura que consumen los handlers.
type Store interface {
	ListComprobantes(ctx context.Context, filter Filter) ([]ComprobanteRow, int, error)
	GetComprobante(ctx context.Context, hash string) (ComprobanteRow, error)
	ListPerfiles(ctx context.Context) ([]Perfil, error)
	LatestTasa(ctx context.Context) (Tasa, error)
	ReportFilters(ctx context.Context) (ReportFilters, error)
}

// Handler atiende las rutas GET /api/v1/*.
type Handler struct {
	store Store
	log   *slog.Logger
}

// NewHandler construye el handler de lectura.
func NewHandler(store Store, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{store: store, log: logger}
}

// Register monta las rutas completas con método en el ServeMux principal.
//
// Go 1.22+ exige registrar el método en el patrón para que ServeMux haga el
// match exacto. Se registran también las variantes con barra final porque
// algunos clientes/proxies la conservan.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/comprobantes", h.ListComprobantes)
	mux.HandleFunc("GET /api/v1/comprobantes/{$}", h.ListComprobantes)
	mux.HandleFunc("GET /api/v1/comprobantes/{hash}", h.GetComprobante)
	mux.HandleFunc("GET /api/v1/comprobantes/{hash}/{$}", h.GetComprobante)
	mux.HandleFunc("GET /api/v1/perfiles", h.ListPerfiles)
	mux.HandleFunc("GET /api/v1/perfiles/{$}", h.ListPerfiles)
	mux.HandleFunc("GET /api/v1/tasas/monitor", h.LatestTasa)
	mux.HandleFunc("GET /api/v1/tasas/monitor/{$}", h.LatestTasa)
	mux.HandleFunc("GET /api/v1/reportes/filtros", h.ReportFilters)
	mux.HandleFunc("GET /api/v1/reportes/filtros/{$}", h.ReportFilters)
}

// ServeHTTP conserva el router de compatibilidad para tests y consumidores
// internos que monten el Handler sin Register.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		h.Options(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}

	path := strings.TrimRight(r.URL.Path, "/")
	switch {
	case path == "/api/v1/comprobantes":
		h.ListComprobantes(w, r)
	case strings.HasPrefix(path, "/api/v1/comprobantes/"):
		h.GetComprobante(w, r)
	case path == "/api/v1/perfiles":
		h.ListPerfiles(w, r)
	case path == "/api/v1/tasas/monitor":
		h.LatestTasa(w, r)
	case path == "/api/v1/reportes/filtros":
		h.ReportFilters(w, r)
	default:
		writeError(w, http.StatusNotFound, "not_found")
	}
}

// Options responde el preflight CORS de las lecturas.
func (h *Handler) Options(w http.ResponseWriter, _ *http.Request) {
	setCORS(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListComprobantes(w http.ResponseWriter, r *http.Request) {
	filter, err := parseFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, total, err := h.store.ListComprobantes(r.Context(), filter)
	if err != nil {
		h.log.Error("hooty.api.comprobantes", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	perfiles, err := h.store.ListPerfiles(r.Context())
	if err != nil {
		h.log.Error("hooty.api.perfiles_para_comprobantes", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	index := MapPerfiles(perfiles)
	items := make([]ComprobanteDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, NewComprobanteDTO(row,
			index[socioKey(row.Socio1)], index[socioKey(row.Socio2)]))
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	w.Header().Set("X-Limit", strconv.Itoa(filter.Limit))
	w.Header().Set("X-Offset", strconv.Itoa(filter.Offset))
	w.Header().Set("X-Has-More", strconv.FormatBool(filter.Offset+len(rows) < total))
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) GetComprobante(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("hash")
	if raw == "" {
		raw = strings.TrimPrefix(r.URL.Path, "/api/v1/comprobantes/")
		raw = strings.TrimSuffix(raw, "/")
	}
	if raw == "" || strings.Contains(raw, "/") {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	hash, err := url.PathUnescape(raw)
	if err != nil || strings.TrimSpace(hash) == "" {
		writeError(w, http.StatusBadRequest, "invalid_hash")
		return
	}
	row, err := h.store.GetComprobante(r.Context(), hash)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		h.log.Error("hooty.api.comprobante", "hash", hash, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	perfiles, err := h.store.ListPerfiles(r.Context())
	if err != nil {
		h.log.Error("hooty.api.perfiles_para_comprobante", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	index := MapPerfiles(perfiles)
	writeJSON(w, http.StatusOK, NewComprobanteDTO(row,
		index[socioKey(row.Socio1)], index[socioKey(row.Socio2)]))
}

func (h *Handler) ListPerfiles(w http.ResponseWriter, r *http.Request) {
	perfiles, err := h.store.ListPerfiles(r.Context())
	if err != nil {
		h.log.Error("hooty.api.perfiles", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	items := make([]PerfilDTO, 0, len(perfiles))
	for _, perfil := range perfiles {
		items = append(items, NewPerfilDTO(perfil))
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) LatestTasa(w http.ResponseWriter, r *http.Request) {
	tasa, err := h.store.LatestTasa(r.Context())
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		h.log.Error("hooty.api.tasa", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, NewTasaDTO(tasa))
}

func (h *Handler) ReportFilters(w http.ResponseWriter, r *http.Request) {
	filters, err := h.store.ReportFilters(r.Context())
	if err != nil {
		h.log.Error("hooty.api.reportes_filtros", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, NewReportFiltersDTO(filters))
}

func parseFilter(r *http.Request) (Filter, error) {
	q := r.URL.Query()
	limit, err := parseLimit(q.Get("limit"), 20, 200)
	if err != nil {
		return Filter{}, err
	}
	offset, err := parseOffset(q.Get("offset"))
	if err != nil {
		return Filter{}, err
	}
	inicio, err := parseDate(q.Get("fechaInicio"))
	if err != nil {
		return Filter{}, err
	}
	fin, err := parseDate(q.Get("fechaFin"))
	if err != nil {
		return Filter{}, err
	}
	if inicio != "" && fin != "" && inicio > fin {
		return Filter{}, errors.New("fechaInicio no puede ser posterior a fechaFin")
	}
	orden := strings.ToLower(strings.TrimSpace(q.Get("orden")))
	switch orden {
	case "", "fecha_desc", "fecha_asc", "monto_desc", "monto_asc":
	default:
		return Filter{}, errors.New("orden inválido")
	}
	if orden == "" {
		orden = "fecha_desc"
	}
	return Filter{
		Socio:       strings.TrimSpace(q.Get("socio")),
		Rol:         strings.TrimSpace(q.Get("rol")),
		FechaInicio: inicio,
		FechaFin:    fin,
		Hash:        strings.TrimSpace(q.Get("hash")),
		Orden:       orden,
		Limit:       limit,
		Offset:      offset,
	}, nil
}

func parseLimit(raw string, fallback, max int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, errors.New("limit inválido")
	}
	if value > max {
		return max, nil
	}
	return value, nil
}

func parseOffset(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, errors.New("offset inválido")
	}
	return value, nil
}

func parseDate(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		return "", errors.New("fecha inválida; use YYYY-MM-DD")
	}
	return raw, nil
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-Id")
	w.Header().Set("Access-Control-Expose-Headers", "X-Total-Count, X-Limit, X-Offset, X-Has-More")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func socioKey(name string) string {
	return domain.Canonical(name)
}
