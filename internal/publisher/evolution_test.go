package publisher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEvolutionSenderEntregaCartelera(t *testing.T) {
	t.Parallel()

	var ruta, apiKey string
	var cuerpo sendMediaRequest
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ruta = r.URL.Path
		apiKey = r.Header.Get("apikey")
		_ = json.NewDecoder(r.Body).Decode(&cuerpo)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer servidor.Close()

	sender, err := NewEvolutionSender(servidor.URL, "clave", "GLAUK", time.Second)
	if err != nil {
		t.Fatalf("sender inválido: %v", err)
	}
	if err := sender.Send(context.Background(), "120363@g.us", []byte("png"), "prueba"); err != nil {
		t.Fatalf("envío rechazado: %v", err)
	}
	if ruta != "/message/sendMedia/GLAUK" {
		t.Fatalf("ruta = %q", ruta)
	}
	if apiKey != "clave" {
		t.Fatalf("apikey = %q", apiKey)
	}
	if cuerpo.Number != "120363@g.us" || cuerpo.MediaType != "image" || cuerpo.Caption != "prueba" {
		t.Fatalf("payload inesperado: %+v", cuerpo)
	}
	if cuerpo.Media != "cG5n" {
		t.Fatalf("media no es base64 puro: %q", cuerpo.Media)
	}
}

func TestEvolutionSenderReportaHTTPError(t *testing.T) {
	t.Parallel()

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("fallo del bot"))
	}))
	defer servidor.Close()

	sender, err := NewEvolutionSender(servidor.URL, "clave", "GLAUK", time.Second)
	if err != nil {
		t.Fatalf("sender inválido: %v", err)
	}
	err = sender.Send(context.Background(), "120363@g.us", []byte("png"), "prueba")
	if err == nil || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "fallo del bot") {
		t.Fatalf("se esperaba error HTTP con cuerpo, obtuve %v", err)
	}
}
