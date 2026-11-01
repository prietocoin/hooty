package r2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResolveImageURLReconstruyeClavePorHash(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead && r.URL.Path == "/comprobantes/abc123.jpg" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	store, err := New(Config{
		Endpoint:     "https://r2.example.test",
		Bucket:       "remesas-img",
		AccessKey:    "access",
		SecretKey:    "secret",
		PublicDomain: server.URL,
		Timeout:      5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.ResolveImageURL(context.Background(), "abc123", "")
	if err != nil {
		t.Fatalf("ResolveImageURL: %v", err)
	}
	want := server.URL + "/comprobantes/abc123.jpg"
	if got != want {
		t.Fatalf("url = %q, se esperaba %q", got, want)
	}
}

func TestPublicURLConservaURLAbsoluta(t *testing.T) {
	t.Parallel()

	store, err := New(Config{
		Endpoint: "https://r2.example.test", Bucket: "bucket",
		AccessKey: "access", SecretKey: "secret", PublicDomain: "https://cdn.example.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := store.PublicURL("https://otro.example/x.jpg"); got != "https://otro.example/x.jpg" {
		t.Fatalf("url absoluta = %q", got)
	}
	if got := store.PublicURL("comprobantes/x.jpg"); got != "https://cdn.example.test/comprobantes/x.jpg" {
		t.Fatalf("url de clave = %q", got)
	}
}
