package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	hootyui "hooty"
	"hooty/internal/api"
)

type routeStore struct{}

func (routeStore) ListComprobantes(context.Context, api.Filter) ([]api.ComprobanteRow, int, error) {
	return []api.ComprobanteRow{{
		HashLargo:      "test-hash",
		HashCorto:      "TESTHASH",
		Instancia:      "hooty",
		MontoRecibo:    10,
		MonedaRecibo:   "USD",
		URLImagen:      "https://cdn.example/test.jpg",
		Socio1:         "CHASAN",
		Socio2:         "NELSY",
		Tasa1:          1,
		Tasa2:          1,
		Monto1:         10,
		Monto2:         10,
		ME1:            10,
		ME2:            10,
		LoteTasa:       "T086",
		FechaOperacion: time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC),
		CreadoEn:       time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC),
		ActualizadoEn:  time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC),
	}}, 1, nil
}

func (routeStore) GetComprobante(context.Context, string) (api.ComprobanteRow, error) {
	return api.ComprobanteRow{}, api.ErrNotFound
}

func (routeStore) UpdateComprobante(context.Context, string, api.ComprobantePatch) (api.ComprobanteRow, error) {
	return api.ComprobanteRow{}, api.ErrNotFound
}

func (routeStore) ListPerfiles(context.Context) ([]api.Perfil, error) {
	return []api.Perfil{
		{Nombre: "CHASAN", Rol: "SOCIO", MonedaBase: "USDT"},
		{Nombre: "NELSY", Rol: "SOCIO", MonedaBase: "USDT"},
	}, nil
}

func (routeStore) LatestTasa(context.Context) (api.Tasa, error) {
	return api.Tasa{IDTasa: "T086", Tasas: json.RawMessage(`{"USD":1}`)}, nil
}

func (routeStore) ReportFilters(context.Context) (api.ReportFilters, error) {
	return api.ReportFilters{Roles: []string{"SOCIO"}}, nil
}

func (routeStore) SaveLote(context.Context, map[string]any) (string, error) {
	return "T999", nil
}

func (routeStore) PublishLote(context.Context) (string, int, error) {
	return "T999", 2, nil
}

func newTestMux(t *testing.T) http.Handler {
	t.Helper()
	uiHandler, err := hootyui.UIHandler()
	if err != nil {
		t.Fatalf("UIHandler: %v", err)
	}
	webhook := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("webhook"))
	})
	health := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("health"))
	})
	readAPI := api.NewHandler(routeStore{}, nil)
	return api.CORS(newMux(webhook, readAPI, health, uiHandler))
}

func TestMuxSirveUIYNoRompeAPI(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(newTestMux(t))
	defer server.Close()

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusPermanentRedirect {
		t.Fatalf("GET / status = %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/comprobantes/" {
		t.Fatalf("Location = %q", location)
	}

	resp, err = http.Get(server.URL + "/comprobantes/")
	if err != nil {
		t.Fatalf("GET /comprobantes/: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /comprobantes/ status = %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "Hooty") {
		t.Fatal("el HTML embebido no contiene Hooty")
	}

	resp, err = http.Get(server.URL + "/api/v1/comprobantes")
	if err != nil {
		t.Fatalf("GET /api/v1/comprobantes: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/comprobantes status = %d, body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("Content-Type = %q", resp.Header.Get("Content-Type"))
	}
	if !strings.Contains(string(body), `"hash_largo":"test-hash"`) {
		t.Fatalf("respuesta API inesperada: %s", body)
	}
}

func TestMuxConservaWebhookYHealth(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(newTestMux(t))
	defer server.Close()

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/healthz", "health"},
		{http.MethodPost, "/webhooks/evolution", "webhook"},
		{http.MethodPost, "/api/v1/webhooks/hooty", "webhook"},
	} {
		req, _ := http.NewRequest(tc.method, server.URL+tc.path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || string(body) != tc.body {
			t.Fatalf("%s %s -> %d %q", tc.method, tc.path, resp.StatusCode, body)
		}
	}
}
