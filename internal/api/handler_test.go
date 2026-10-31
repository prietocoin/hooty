package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hooty/internal/ingest"
	"hooty/internal/publisher"
)

type fakeStore struct {
	rows       []ComprobanteRow
	total      int
	perfiles   []Perfil
	tasa       Tasa
	filters    ReportFilters
	last       Filter
	savedTasas map[string]any
	deletedID  string
	listErr    error
	getErr     error
	perfilErr  error
	tasaErr    error
	saveErr    error
	publishErr error
	deleteErr  error
}

func (f *fakeStore) ListComprobantes(_ context.Context, filter Filter) ([]ComprobanteRow, int, error) {
	f.last = filter
	return f.rows, f.total, f.listErr
}

func (f *fakeStore) GetComprobante(_ context.Context, _ string) (ComprobanteRow, error) {
	if f.getErr != nil {
		return ComprobanteRow{}, f.getErr
	}
	if len(f.rows) == 0 {
		return ComprobanteRow{}, ErrNotFound
	}
	return f.rows[0], nil
}

func (f *fakeStore) UpdateComprobante(_ context.Context, _ string, _ ComprobantePatch) (ComprobanteRow, error) {
	if f.getErr != nil {
		return ComprobanteRow{}, f.getErr
	}
	if len(f.rows) == 0 {
		return ComprobanteRow{}, ErrNotFound
	}
	return f.rows[0], nil
}

func (f *fakeStore) ListPerfiles(_ context.Context) ([]Perfil, error) {
	return f.perfiles, f.perfilErr
}

func (f *fakeStore) LatestTasa(_ context.Context) (Tasa, error) {
	return f.tasa, f.tasaErr
}

func (f *fakeStore) SaveLote(_ context.Context, tasas map[string]any) (string, error) {
	f.savedTasas = tasas
	return "T999", f.saveErr
}

func (f *fakeStore) PublishLote(_ context.Context) (string, int, error) {
	if f.publishErr != nil {
		return "", 0, f.publishErr
	}
	return "T999", 2, nil
}

func (f *fakeStore) DeleteLote(_ context.Context, id string) (bool, error) {
	f.deletedID = id
	if f.deleteErr != nil {
		return false, f.deleteErr
	}
	return id != "T000", nil
}

func (f *fakeStore) ReportFilters(_ context.Context) (ReportFilters, error) {
	return f.filters, nil
}

func testStore() *fakeStore {
	return &fakeStore{
		rows: []ComprobanteRow{{
			HashLargo:      "8a3e",
			HashCorto:      "8A3E",
			Instancia:      "hooty",
			MontoRecibo:    100,
			MonedaRecibo:   "USD",
			Banco:          "BANCO",
			Titular:        "TITULAR",
			Referencia:     "REF",
			URLImagen:      "https://cdn.example/imagen.jpg",
			Grupo1:         "111@g.us",
			Grupo2:         "222@g.us",
			Socio1:         "CHASAN",
			Socio2:         "NELSY",
			Tasa1:          40,
			Tasa2:          41,
			Monto1:         4000,
			Monto2:         4100,
			ME1:            100,
			ME2:            100,
			LoteTasa:       "T070",
			FechaOperacion: time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC),
			CreadoEn:       time.Date(2026, 10, 25, 10, 1, 0, 0, time.UTC),
			ActualizadoEn:  time.Date(2026, 10, 25, 10, 2, 0, 0, time.UTC),
		}},
		total: 1,
		perfiles: []Perfil{
			{Nombre: "CHASAN", MonedaBase: "VES", Rol: "SOCIO", Monedas: json.RawMessage(`{"USD":{"tipo":"P"}}`)},
			{Nombre: "NELSY", MonedaBase: "COP", Rol: "SOCIO"},
		},
		tasa: Tasa{IDTasa: "T070", Tasas: json.RawMessage(`{"USD":1,"VES":40}`)},
		filters: ReportFilters{
			Roles:     []string{"SOCIO"},
			Entidades: []string{"CHASAN", "NELSY"},
			Hashes:    []HashOption{{Value: "8a3e", Label: "8A3E"}},
		},
	}
}

func newTestHandler(store *fakeStore) *Handler {
	return NewHandler(store, nil)
}

func TestListComprobantesDTOYPaginacion(t *testing.T) {
	t.Parallel()

	store := testStore()
	handler := newTestHandler(store)
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/comprobantes?socio=chasan&fechaInicio=2026-10-01&fechaFin=2026-10-31&limit=20&offset=0", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Total-Count"); got != "1" {
		t.Fatalf("X-Total-Count = %q", got)
	}
	if got := rec.Header().Get("X-Has-More"); got != "false" {
		t.Fatalf("X-Has-More = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got == "" {
		t.Fatal("faltan cabeceras CORS expuestas")
	}
	if store.last.Socio != "chasan" || store.last.FechaInicio != "2026-10-01" || store.last.FechaFin != "2026-10-31" {
		t.Fatalf("filtros no propagados: %+v", store.last)
	}

	var items []ComprobanteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	item := items[0]
	if item.Monto != 100 || item.MontoRecibo != 100 || item.TipoOp1 != "P" || item.TipoOp2 != "D" {
		t.Fatalf("DTO inesperado: %+v", item)
	}
	if item.FechaOperacion != "2026-10-25T10:00:00" || item.FechaHoraComprobante != item.FechaOperacion {
		t.Fatalf("fecha inesperada: %+v", item)
	}
}

func TestListComprobantesRechazaFechaInvalida(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comprobantes?fechaInicio=2026-99-99", nil)
	rec := httptest.NewRecorder()
	newTestHandler(testStore()).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetComprobanteYNoEncontrado(t *testing.T) {
	t.Parallel()

	store := testStore()
	handler := newTestHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/comprobantes/8a3e", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	empty := testStore()
	empty.getErr = ErrNotFound
	req = httptest.NewRequest(http.MethodGet, "/api/v1/comprobantes/nope", nil)
	rec = httptest.NewRecorder()
	newTestHandler(empty).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestPerfilesTasaYFiltros(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(testStore())

	cases := []struct {
		path string
		key  string
	}{
		{"/api/v1/perfiles", "whatsapp"},
		{"/api/v1/tasas/monitor", "id_tasa"},
		{"/api/v1/reportes/filtros", "roles"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", tc.path, rec.Code, rec.Body.String())
		}
		if tc.path == "/api/v1/perfiles" {
			var profiles []map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &profiles); err != nil || len(profiles) == 0 {
				t.Fatalf("perfiles inválidos: %v, body=%s", err, rec.Body.String())
			}
			if profiles[0][tc.key] == nil {
				t.Fatalf("perfil sin %s: %s", tc.key, rec.Body.String())
			}
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("%s JSON inválido: %v", tc.path, err)
		}
		if payload[tc.key] == nil {
			t.Fatalf("respuesta sin %s: %s", tc.key, rec.Body.String())
		}
	}
}

func TestRegisterRutasExactasYBarraFinal(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	newTestHandler(testStore()).Register(mux)

	for _, path := range []string{
		"/api/v1/comprobantes",
		"/api/v1/comprobantes/",
		"/api/v1/comprobantes/8a3e",
		"/api/v1/comprobantes/8a3e/",
		"/api/v1/perfiles",
		"/api/v1/perfiles/",
		"/api/v1/tasas/monitor",
		"/api/v1/tasas/monitor/",
		"/api/v1/reportes/filtros",
		"/api/v1/reportes/filtros/",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body=%s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestUpdateComprobanteAceptaPUT(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	newTestHandler(testStore()).Register(mux)

	body := `{
		"banco":"BINANCE",
		"referencia":"REF-1",
		"titular":"CHASAN",
		"monto_recibo":100,
		"moneda_recibo":"USD",
		"socio_1":"CHASAN","monto_1":100,"tasa_1":1,"me1":100,
		"socio_2":"NELSY","monto_2":100,"tasa_2":1,"me2":100
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/comprobantes/8a3e", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var dto ComprobanteDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("respuesta PUT inválida: %v", err)
	}
	if dto.HashLargo != "8a3e" {
		t.Fatalf("hash = %q", dto.HashLargo)
	}
}

func TestUpdateComprobanteRechazaJSONInvalido(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	newTestHandler(testStore()).Register(mux)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/comprobantes/8a3e", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT inválido status = %d", rec.Code)
	}
}

func TestApplyComprobantePatchActualizaAmbosLados(t *testing.T) {
	t.Parallel()

	c := comprobanteFromRow(testStore().rows[0])
	banco := "OTRO BANCO"
	referencia := "REF-NUEVA"
	titular := "TITULAR NUEVO"
	montoRecibo := 12.34
	moneda := "PEN"
	socio1 := "CHASAN"
	monto1 := 45.67
	tasa1 := 3.7
	me1 := 12.34
	socio2 := "NELSY"
	monto2 := 48.9
	tasa2 := 3.9
	me2 := 12.54

	patch := ComprobantePatch{
		Banco:        &banco,
		Referencia:   &referencia,
		Titular:      &titular,
		MontoRecibo:  &montoRecibo,
		MonedaRecibo: &moneda,
		Socio1:       &socio1,
		Monto1:       &monto1,
		Tasa1:        &tasa1,
		ME1:          &me1,
		Socio2:       &socio2,
		Monto2:       &monto2,
		Tasa2:        &tasa2,
		ME2:          &me2,
	}
	if err := applyComprobantePatch(&c, patch); err != nil {
		t.Fatalf("patch válido rechazado: %v", err)
	}
	if c.Banco != banco || c.Referencia != referencia || c.Titular != titular ||
		c.MontoRecibo != montoRecibo || c.MonedaRecibo != moneda ||
		c.Socio1 != socio1 || c.Monto1 != monto1 || c.Tasa1 != tasa1 || c.ME1 != me1 ||
		c.Socio2 != socio2 || c.Monto2 != monto2 || c.Tasa2 != tasa2 || c.ME2 != me2 {
		t.Fatalf("patch no aplicado correctamente: %+v", c)
	}
	if c.Grupo1 != "111@g.us" || c.Grupo2 != "222@g.us" || c.LoteTasa != "T070" {
		t.Fatalf("el patch alteró campos inmutables: %+v", c)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("el comprobante editado debe ser válido: %v", err)
	}
}

func TestApplyComprobantePatchRechazaMontoReciboNegativo(t *testing.T) {
	t.Parallel()

	c := comprobanteFromRow(testStore().rows[0])
	monto := -1.0
	if err := applyComprobantePatch(&c, ComprobantePatch{MontoRecibo: &monto}); err == nil {
		t.Fatal("monto_recibo negativo debe rechazarse")
	}
}

func TestReconcileSideCalculaYEvaluaSigno(t *testing.T) {
	t.Parallel()

	monto := 0.0
	me := 0.0
	if err := reconcileSide("1", 100, 40, &monto, &me, 3.7, nil); err != nil {
		t.Fatalf("cálculo válido rechazado: %v", err)
	}
	if monto != 2.5 || me != 0.67 {
		t.Fatalf("monto=%v me=%v, se esperaba 2.5/0.67", monto, me)
	}

	negMonto := -2.5
	if err := reconcileSide("1", 100, 40, &monto, &me, 3.7, &negMonto); err != nil {
		t.Fatalf("signo negativo coherente rechazado: %v", err)
	}
	if monto != -2.5 || me != -0.67 {
		t.Fatalf("signo no preservado: monto=%v me=%v", monto, me)
	}

	manual := -2.4
	if err := reconcileSide("1", 100, 40, &monto, &me, 3.7, &manual); err != nil {
		t.Fatalf("excepción manual rechazada: %v", err)
	}
	if monto != -2.4 || me != -0.64 {
		t.Fatalf("excepción manual no respetada: monto=%v me=%v", monto, me)
	}
}

func TestReconcileSideAceptaTasaCero(t *testing.T) {
	t.Parallel()

	monto := 12.5
	me := 3.4
	if err := reconcileSide("2", 100, 0, &monto, &me, 3.7, nil); err != nil {
		t.Fatalf("una tasa 0 debe aceptarse: %v", err)
	}
	if monto != 0 || me != 0 {
		t.Fatalf("el lado sin tasa debe quedar en cero: monto=%v me=%v", monto, me)
	}
	if err := reconcileSide("2", 100, -1, &monto, &me, 3.7, nil); err == nil {
		t.Fatal("una tasa negativa debe rechazarse")
	}
}

type fakeSimulator struct {
	result ingest.SimulacionResult
	err    error
}

func (f fakeSimulator) Simular(context.Context, ingest.SimulacionInput) (ingest.SimulacionResult, error) {
	return f.result, f.err
}

func TestSimularComprobanteUsaMotorCanonico(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	NewHandler(testStore(), nil, fakeSimulator{result: ingest.SimulacionResult{
		Tasa1: 1629, Tasa2: 1596, Monto1: 30.07, Monto2: 30.07, ME1: 30.07, ME2: 30.07,
	}}).Register(mux)

	body := `{"moneda_recibo":"ARS","monto_recibo":49000,"socio_1":"CHASAN","socio_2":"NELSY","lote_tasa":"T070"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comprobantes/simular", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST simular status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var result ingest.SimulacionResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("respuesta de simulación inválida: %v", err)
	}
	if result.Tasa1 != 1629 || result.Tasa2 != 1596 {
		t.Fatalf("tasas de simulación inesperadas: %+v", result)
	}
}

func TestTasasGuardarYPublicarUsanPUT(t *testing.T) {
	t.Parallel()

	store := testStore()
	mux := http.NewServeMux()
	NewHandler(store, nil).Register(mux)

	body := `{"tasas":{"ARS":1622,"COP":3094}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/tasas/guardar-lote", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT guardar-lote status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.savedTasas["ARS"] != float64(1622) {
		t.Fatalf("tasas no guardadas: %+v", store.savedTasas)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/tasas/guardar-lote", strings.NewReader(body))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST guardar-lote status = %d, se esperaba 405", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/tasas/publicar-lote", strings.NewReader(`{}`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT publicar-lote status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var publicado struct {
		IDTasa     string `json:"id_tasa"`
		Procesados int    `json:"procesados"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &publicado); err != nil {
		t.Fatalf("respuesta de publicación inválida: %v", err)
	}
	if publicado.IDTasa != "T999" || publicado.Procesados != 2 {
		t.Fatalf("publicación inesperada: %+v", publicado)
	}
}

type fakePublisher struct {
	prueba bool
	calls  int
	err    error
}

func (f *fakePublisher) Publish(_ context.Context, isPrueba bool) (publisher.Result, error) {
	f.prueba = isPrueba
	f.calls++
	if f.err != nil {
		return publisher.Result{}, f.err
	}
	return publisher.Result{IDTasa: "T999", Procesados: 1, Prueba: isPrueba}, nil
}

func TestPublicarLoteLeeIsPruebaYDespacha(t *testing.T) {
	t.Parallel()

	despachador := &fakePublisher{}
	mux := http.NewServeMux()
	NewHandler(testStore(), nil).WithPublisher(despachador).Register(mux)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/tasas/publicar-lote",
		strings.NewReader(`{"is_prueba":true,"modo_prueba_global":false}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("publicar status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if despachador.calls != 1 || !despachador.prueba {
		t.Fatalf("is_prueba no llegó al despachador: %+v", despachador)
	}
}

func TestDeleteLoteUsaDELETE(t *testing.T) {
	t.Parallel()

	store := testStore()
	mux := http.NewServeMux()
	NewHandler(store, nil).Register(mux)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasas/lote/T096", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE lote status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.deletedID != "T096" {
		t.Fatalf("id borrado = %q", store.deletedID)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/tasas/lote/T000", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("DELETE inexistente status = %d, se esperaba 404", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/tasas/lote/T096", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST lote status = %d, se esperaba 405", rec.Code)
	}
}

func TestServidorHTTPResponde200EnComprobantes(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	newTestHandler(testStore()).Register(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/comprobantes")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Total-Count"); got != "1" {
		t.Fatalf("X-Total-Count = %q", got)
	}
}

func TestRegisterNoPaniegaConWebhookYCORSGlobal(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.Handle("/api/v1/webhooks/hooty", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	newTestHandler(testStore()).Register(mux)
	server := httptest.NewServer(CORS(mux))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodOptions, server.URL+"/api/v1/comprobantes", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("CORS = %q", resp.Header.Get("Access-Control-Allow-Origin"))
	}

	resp, err = http.Get(server.URL + "/api/v1/comprobantes")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d", resp.StatusCode)
	}
}

func TestRegisterRechazaMetodoNoPermitido(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	newTestHandler(testStore()).Register(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comprobantes", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestOPTIONSCORS(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/comprobantes", nil)
	rec := httptest.NewRecorder()
	newTestHandler(testStore()).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("CORS = %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestErrorInternoNoFiltraSQL(t *testing.T) {
	t.Parallel()

	store := testStore()
	store.listErr = errors.New(`pq: relation "comprobantes" does not exist`)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/comprobantes", nil)
	rec := httptest.NewRecorder()
	newTestHandler(store).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	var payload map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload["error"] != "internal_error" {
		t.Fatalf("error expuesto: %s", rec.Body.String())
	}
}
