package hooty

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUIHandlerSirveLaRaizYLosAssets(t *testing.T) {
	t.Parallel()

	handler, err := UIHandler()
	if err != nil {
		t.Fatalf("UIHandler: %v", err)
	}

	t.Run("raiz redirige a comprobantes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusPermanentRedirect {
			t.Fatalf("status = %d", rec.Code)
		}
		if location := rec.Header().Get("Location"); location != "/comprobantes/" {
			t.Fatalf("Location = %q", location)
		}
	})

	t.Run("index de comprobantes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/comprobantes/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		body, _ := io.ReadAll(rec.Body)
		if !strings.Contains(string(body), "Fundablock") {
			t.Fatal("no se sirvió el index de comprobantes")
		}
		if rec.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
		}
	})

	t.Run("config y vendor", func(t *testing.T) {
		for _, path := range []string{"/config.js", "/shared/js/api-base.js"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s status = %d", path, rec.Code)
			}
		}
	})

	t.Run("metodo no permitido", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/comprobantes/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestMuxEstaticoNoColisionaConAPIYWebhooks(t *testing.T) {
	t.Parallel()

	uiHandler, err := UIHandler()
	if err != nil {
		t.Fatalf("UIHandler: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("health"))
	})
	mux.HandleFunc("POST /webhooks/evolution", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("webhook"))
	})
	mux.HandleFunc("GET /api/v1/comprobantes", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("api"))
	})
	mux.Handle("/", uiHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	cases := []struct {
		method string
		path   string
		status int
		body   string
	}{
		{http.MethodGet, "/healthz", http.StatusOK, "health"},
		{http.MethodPost, "/webhooks/evolution", http.StatusOK, "webhook"},
		{http.MethodGet, "/api/v1/comprobantes", http.StatusOK, "api"},
		{http.MethodGet, "/comprobantes/", http.StatusOK, "Fundablock"},
	}
	for _, tc := range cases {
		req, _ := http.NewRequest(tc.method, server.URL+tc.path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tc.status || !strings.Contains(string(body), tc.body) {
			t.Fatalf("%s %s -> %d %q", tc.method, tc.path, resp.StatusCode, body)
		}
	}
}
