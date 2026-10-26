package r2

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSubirFirmaYPublicaURLDeterminista(t *testing.T) {
	t.Parallel()

	var gotPath, gotMethod, gotAuth, gotContentType, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store, err := New(Config{
		Endpoint:     server.URL,
		Bucket:       "bucket",
		AccessKey:    "access",
		SecretKey:    "secret",
		PublicDomain: "https://cdn.example/",
		Region:       "auto",
		Now: func() time.Time {
			return time.Date(2026, 10, 25, 15, 4, 5, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	url, err := store.Subir(context.Background(), "/comprobantes/abc.jpg", []byte("imagen"), "image/jpeg")
	if err != nil {
		t.Fatalf("Subir: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/bucket/comprobantes/abc.jpg" {
		t.Fatalf("petición inesperada: %s %s", gotMethod, gotPath)
	}
	if gotBody != "imagen" || gotContentType != "image/jpeg" {
		t.Fatalf("cuerpo/headers inesperados: body=%q content-type=%q", gotBody, gotContentType)
	}
	for _, fragment := range []string{
		"AWS4-HMAC-SHA256 Credential=access/20261025/auto/s3/aws4_request",
		"SignedHeaders=host;x-amz-content-sha256;x-amz-date",
		"Signature=",
	} {
		if !strings.Contains(gotAuth, fragment) {
			t.Fatalf("Authorization no contiene %q: %s", fragment, gotAuth)
		}
	}
	if url != "https://cdn.example/comprobantes/abc.jpg" {
		t.Fatalf("URL pública = %q", url)
	}
}

func TestNewRechazaConfiguracionIncompleta(t *testing.T) {
	t.Parallel()

	if _, err := New(Config{Endpoint: "https://r2.example"}); err == nil {
		t.Fatal("se esperaba error por bucket y credenciales")
	}
}
