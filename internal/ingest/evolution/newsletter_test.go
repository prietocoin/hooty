package evolution

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResolveNewsletterJIDDesdeMetadata(t *testing.T) {
	t.Parallel()

	const code = "0029Vb7690K7z4kZ7c3Hz128"
	const jid = "120363000000000000@newsletter"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/getNewsletterMetadata/glauk" {
			t.Errorf("ruta inesperada: %s", r.URL.Path)
		}
		if r.URL.Query().Get("inviteCode") != code {
			t.Errorf("inviteCode = %q", r.URL.Query().Get("inviteCode"))
		}
		if r.Header.Get("apikey") != "secret" {
			t.Errorf("apikey = %q", r.Header.Get("apikey"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + jid + `","invite":"` + code + `"}`))
	}))
	defer server.Close()

	client, err := New(Config{BaseURL: server.URL, APIKey: "secret", Instance: "glauk", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.ResolveNewsletterJID(context.Background(), code)
	if err != nil {
		t.Fatalf("ResolveNewsletterJID: %v", err)
	}
	if got != jid {
		t.Fatalf("jid = %q, se esperaba %q", got, jid)
	}
}

func TestResolveNewsletterJIDCaeAListaCompleta(t *testing.T) {
	t.Parallel()

	const code = "0029Vb7690K7z4kZ7c3Hz128"
	const jid = "120363111111111111@newsletter"

	var metadataCalls, listCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chat/getNewsletterMetadata/glauk":
			metadataCalls++
			http.NotFound(w, r)
		case "/chat/findNewsletters/glauk":
			listCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"` + jid + `","invite":"https://whatsapp.com/channel/` + code + `"}]}`))
		default:
			t.Errorf("ruta inesperada: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := New(Config{BaseURL: server.URL, APIKey: "secret", Instance: "glauk", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.ResolveNewsletterJID(context.Background(), code)
	if err != nil {
		t.Fatalf("ResolveNewsletterJID: %v", err)
	}
	if got != jid {
		t.Fatalf("jid = %q, se esperaba %q", got, jid)
	}
	if metadataCalls != 1 || listCalls != 1 {
		t.Fatalf("llamadas metadata=%d lista=%d", metadataCalls, listCalls)
	}
}

func TestResolveNewsletterJIDRechazaCodigoVacio(t *testing.T) {
	t.Parallel()

	client, err := New(Config{BaseURL: "https://example.test", APIKey: "secret", Instance: "glauk", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ResolveNewsletterJID(context.Background(), "  "); err == nil ||
		!strings.Contains(err.Error(), "inviteCode") {
		t.Fatalf("se esperaba error por inviteCode vacío, got %v", err)
	}
}
