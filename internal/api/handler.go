package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"hooty/internal/domain"
	"hooty/internal/ingest"
	"hooty/internal/publisher"
)

// Store es la superficie de lectura que consumen los handlers.
type Store interface {
	ListComprobantes(ctx context.Context, filter Filter) ([]ComprobanteRow, int, error)
	GetComprobante(ctx context.Context, hash string) (ComprobanteRow, error)
	UpdateComprobante(ctx context.Context, hash string, patch ComprobantePatch) (ComprobanteRow, error)
	ListPerfiles(ctx context.Context) ([]Perfil, error)
	LatestTasa(ctx context.Context) (Tasa, error)
	SaveLote(ctx context.Context, tasas map[string]any) (string, error)
	PublishLote(ctx context.Context) (string, int, error)
	DeleteLote(ctx context.Context, id string) (bool, error)
	ReportFilters(ctx context.Context) (ReportFilters, error)
}

// Handler atiende las rutas GET /api/v1/*.
type Handler struct {
	store     Store
	log       *slog.Logger
	simulator Simulator
	publisher RatePublisher
}

// RatePublisher despacha las carteleras del lote vigente.
type RatePublisher interface {
	Publish(ctx context.Context, isPrueba bool) (publisher.Result, error)
}

// ImagePublisher recibe las carteleras que el frontend generó con html2canvas.
type ImagePublisher interface {
	PublishImages(ctx context.Context, isPrueba bool, envios []publisher.Envio) (publisher.Result, error)
}

// Previewer renderiza una cartelera individual con el mismo generador oficial.
type Previewer interface {
	Preview(ctx context.Context, socio string, prueba bool) ([]byte, error)
}

// SinglePublisher despacha la cartelera de un socio puntual.
type SinglePublisher interface {
	PublishSocio(ctx context.Context, socio string, isPrueba bool) (publisher.Result, error)
}

// Simulator expone el motor de liquidación canónico para el modal de edición.
type Simulator interface {
	Simular(ctx context.Context, input ingest.SimulacionInput) (ingest.SimulacionResult, error)
}

// NewHandler construye el handler de lectura.
func NewHandler(store Store, logger *slog.Logger, simuladores ...Simulator) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	handler := &Handler{store: store, log: logger}
	if len(simuladores) > 0 {
		handler.simulator = simuladores[0]
	}
	return handler
}

// WithPublisher conecta el motor de despacho real. Sin él, PublishLote conserva
// el modo de compatibilidad que solo cuenta socios.
func (h *Handler) WithPublisher(p RatePublisher) *Handler {
	h.publisher = p
	return h
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
	mux.HandleFunc("PUT /api/v1/comprobantes/{hash}", h.UpdateComprobante)
	mux.HandleFunc("PUT /api/v1/comprobantes/{hash}/{$}", h.UpdateComprobante)
	mux.HandleFunc("POST /api/v1/comprobantes/simular", h.Simular)
	mux.HandleFunc("GET /api/v1/perfiles", h.ListPerfiles)
	mux.HandleFunc("GET /api/v1/perfiles/{$}", h.ListPerfiles)
	mux.HandleFunc("GET /api/v1/tasas/monitor", h.LatestTasa)
	mux.HandleFunc("GET /api/v1/tasas/monitor/{$}", h.LatestTasa)
	mux.HandleFunc("GET /api/v1/tasas/preview", h.PreviewLote)
	mux.HandleFunc("GET /api/v1/tasas/preview/{socio}", h.PreviewLote)
	mux.HandleFunc("PUT /api/v1/tasas/guardar-lote", h.SaveLote)
	mux.HandleFunc("PUT /api/v1/tasas/guardar-lote/{$}", h.SaveLote)
	mux.HandleFunc("PUT /api/v1/tasas/publicar-lote", h.PublishLote)
	mux.HandleFunc("PUT /api/v1/tasas/publicar-lote/{$}", h.PublishLote)
	mux.HandleFunc("PUT /api/v1/tasas/disparar", h.PublishSocio)
	mux.HandleFunc("PUT /api/v1/tasas/disparar/{$}", h.PublishSocio)
	mux.HandleFunc("DELETE /api/v1/tasas/lote/{id}", h.DeleteLote)
	mux.HandleFunc("DELETE /api/v1/tasas/lote/{id}/{$}", h.DeleteLote)
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
	if r.Method != http.MethodGet && r.Method != http.MethodPut &&
		r.Method != http.MethodPost && r.Method != http.MethodDelete {
		w.Header().Set("Allow", "GET, PUT, POST, DELETE, OPTIONS")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}

	path := strings.TrimRight(r.URL.Path, "/")
	switch {
	case r.Method == http.MethodPost && path == "/api/v1/comprobantes/simular":
		h.Simular(w, r)
	case r.Method == http.MethodPost:
		w.Header().Set("Allow", "GET, PUT, POST, DELETE, OPTIONS")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
	case r.Method == http.MethodPut && strings.HasPrefix(path, "/api/v1/comprobantes/"):
		h.UpdateComprobante(w, r)
	case r.Method == http.MethodPut && path == "/api/v1/tasas/guardar-lote":
		h.SaveLote(w, r)
	case r.Method == http.MethodPut && path == "/api/v1/tasas/publicar-lote":
		h.PublishLote(w, r)
	case r.Method == http.MethodPut && path == "/api/v1/tasas/disparar":
		h.PublishSocio(w, r)
	case r.Method == http.MethodPut:
		w.Header().Set("Allow", "GET, PUT, DELETE, OPTIONS")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/tasas/lote/"):
		h.DeleteLote(w, r)
	case r.Method == http.MethodDelete:
		w.Header().Set("Allow", "GET, PUT, POST, DELETE, OPTIONS")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
	case path == "/api/v1/comprobantes":
		h.ListComprobantes(w, r)
	case strings.HasPrefix(path, "/api/v1/comprobantes/"):
		h.GetComprobante(w, r)
	case path == "/api/v1/perfiles":
		h.ListPerfiles(w, r)
	case path == "/api/v1/tasas/monitor":
		h.LatestTasa(w, r)
	case path == "/api/v1/tasas/preview" || strings.HasPrefix(path, "/api/v1/tasas/preview/"):
		h.PreviewLote(w, r)
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

// Simular resuelve tasas y montos con el motor canónico, sin persistir.
func (h *Handler) Simular(w http.ResponseWriter, r *http.Request) {
	if h.simulator == nil {
		writeError(w, http.StatusServiceUnavailable, "simulator_unavailable")
		return
	}
	var input ingest.SimulacionInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	result, err := h.simulator.Simular(r.Context(), input)
	if errors.Is(err, domain.ErrValidation) {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		h.log.Error("hooty.api.comprobante_simular", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// UpdateComprobante edita cabecera y ambos lados del binomio sin tocar la
// identidad (hash, grupos, lote, fecha de operación ni URL de imagen).
func (h *Handler) UpdateComprobante(w http.ResponseWriter, r *http.Request) {
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

	var patch ComprobantePatch
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	row, err := h.store.UpdateComprobante(r.Context(), hash, patch)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if errors.Is(err, domain.ErrValidation) {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		h.log.Error("hooty.api.comprobante_update", "hash", hash, "error", err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// Los CHECK/FK/UNIQUE se devuelven con el mensaje de PostgreSQL
			// para que la UI pueda mostrar la causa exacta al operador.
			writeError(w, http.StatusUnprocessableEntity, pgErr.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	perfiles, err := h.store.ListPerfiles(r.Context())
	if err != nil {
		h.log.Error("hooty.api.perfiles_para_edicion", "error", err)
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

// SaveLote responde PUT /api/v1/tasas/guardar-lote. Inserta un lote nuevo en
// tasas_glaukov y devuelve el identificador asignado.
func (h *Handler) SaveLote(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Tasas map[string]any `json:"tasas"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(&body); err != nil || len(body.Tasas) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_rate_lote")
		return
	}
	tasas := make(map[string]any, len(body.Tasas))
	for clave, valor := range body.Tasas {
		switch typed := valor.(type) {
		case float64:
			if typed >= 0 && !math.IsNaN(typed) && !math.IsInf(typed, 0) {
				tasas[clave] = typed
			}
		case string:
			if strings.EqualFold(clave, "CREATED_AT") {
				tasas[clave] = typed
			}
		}
	}
	if len(tasas) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_rate_lote")
		return
	}
	if _, ok := tasas["CREATED_AT"]; !ok {
		tasas["CREATED_AT"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	}
	id, err := h.store.SaveLote(r.Context(), tasas)
	if err != nil {
		h.log.Error("hooty.api.tasas_guardar", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id_tasa": id})
}

// PublishLote responde PUT /api/v1/tasas/publicar-lote. Acepta las carteleras
// ya rasterizadas por el frontend y, si no llegan, conserva el fallback de
// renderizado del servidor.
func (h *Handler) PublishLote(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IsPrueba         *bool `json:"is_prueba"`
		ModoPruebaGlobal *bool `json:"modo_prueba_global"`
		Carteleras       []struct {
			Socio  string `json:"socio"`
			ChatID string `json:"chat_id"`
			Imagen string `json:"imagen"`
		} `json:"carteleras"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<20))
		if err := decoder.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_publish_payload")
			return
		}
	}
	isPrueba := false
	if body.IsPrueba != nil {
		isPrueba = *body.IsPrueba
	} else if body.ModoPruebaGlobal != nil {
		isPrueba = *body.ModoPruebaGlobal
	}

	envios := make([]publisher.Envio, 0, len(body.Carteleras))
	for _, cartelera := range body.Carteleras {
		imagen := strings.TrimSpace(cartelera.Imagen)
		if comma := strings.Index(imagen, ","); strings.HasPrefix(imagen, "data:") && comma >= 0 {
			imagen = imagen[comma+1:]
		}
		png, err := base64.StdEncoding.DecodeString(imagen)
		if err != nil || len(png) == 0 {
			writeError(w, http.StatusBadRequest, "invalid_cartelera_image")
			return
		}
		envios = append(envios, publisher.Envio{
			Socio: cartelera.Socio, ChatID: cartelera.ChatID, PNG: png,
		})
	}

	if h.publisher != nil {
		var (
			resultado publisher.Result
			err       error
		)
		if len(envios) > 0 {
			if images, ok := h.publisher.(ImagePublisher); ok {
				resultado, err = images.PublishImages(r.Context(), isPrueba, envios)
			} else {
				resultado, err = h.publisher.Publish(r.Context(), isPrueba)
			}
		} else {
			resultado, err = h.publisher.Publish(r.Context(), isPrueba)
		}
		if err != nil {
			h.log.Error("hooty.api.tasas_publicar", "prueba", isPrueba, "error", err)
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resultado)
		return
	}

	id, procesados, err := h.store.PublishLote(r.Context())
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusBadRequest, "no hay lote vigente")
		return
	}
	if err != nil {
		h.log.Error("hooty.api.tasas_publicar", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id_tasa":    id,
		"procesados": procesados,
		"prueba":     isPrueba,
	})
}

// DeleteLote responde DELETE /api/v1/tasas/lote/{id}. Elimina el lote físico y
// devuelve 404 si el identificador no existe.
// PublishSocio responde PUT /api/v1/tasas/disparar. Renderiza y envía la
// cartelera del socio indicado; el modo prueba solo cambia el destino.
func (h *Handler) PublishSocio(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Socio            string `json:"socio"`
		IsPrueba         *bool  `json:"is_prueba"`
		ModoPrueba       *bool  `json:"modo_prueba"`
		ModoPruebaGlobal *bool  `json:"modo_prueba_global"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_publish_payload")
		return
	}
	if strings.TrimSpace(body.Socio) == "" {
		writeError(w, http.StatusBadRequest, "socio es obligatorio")
		return
	}
	isPrueba := false
	for _, flag := range []*bool{body.IsPrueba, body.ModoPrueba, body.ModoPruebaGlobal} {
		if flag != nil {
			isPrueba = *flag
			break
		}
	}
	despachador, ok := h.publisher.(SinglePublisher)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "publisher_unavailable")
		return
	}
	resultado, err := despachador.PublishSocio(r.Context(), body.Socio, isPrueba)
	if err != nil {
		h.log.Error("hooty.api.tasas_disparar", "socio", body.Socio, "prueba", isPrueba, "error", err)
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resultado)
}

// PreviewLote responde GET /api/v1/tasas/preview/{socio} con el PNG generado
// por el mismo motor oficial que usa la publicación masiva.
func (h *Handler) PreviewLote(w http.ResponseWriter, r *http.Request) {
	previewer, ok := h.publisher.(Previewer)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "preview_unavailable")
		return
	}
	socio := strings.TrimSpace(r.PathValue("socio"))
	if socio == "" {
		socio = strings.TrimSpace(r.URL.Query().Get("socio"))
	}
	prueba, _ := strconv.ParseBool(r.URL.Query().Get("prueba"))
	png, err := previewer.Preview(r.Context(), socio, prueba)
	if err != nil {
		h.log.Error("hooty.api.tasas_preview", "socio", socio, "prueba", prueba, "error", err)
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (h *Handler) DeleteLote(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		id = strings.TrimPrefix(strings.TrimRight(r.URL.Path, "/"), "/api/v1/tasas/lote/")
	}
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	borrado, err := h.store.DeleteLote(r.Context(), id)
	if err != nil {
		h.log.Error("hooty.api.tasas_borrar", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if !borrado {
		writeError(w, http.StatusNotFound, "lote no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Lote " + id + " eliminado correctamente.",
	})
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
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-Id, Authorization, X-Hooty-Token")
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
