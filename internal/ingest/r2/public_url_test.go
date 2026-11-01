package r2

import "testing"

func TestPublicURLPriorizaR2PublicBaseURL(t *testing.T) {
	t.Setenv("R2_PUBLIC_BASE_URL", "https://pub-49b9c87f6e6a418ba42de5ba36ddc73e.r2.dev/")
	t.Setenv("R2_PUBLIC_DOMAIN", "https://ignorado.example")

	store, err := New(Config{
		Endpoint: "https://19e79842547693ffe34fbd1d311d25dc.r2.cloudflarestorage.com",
		Bucket:   "remesas-img", AccessKey: "access", SecretKey: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	want := "https://pub-49b9c87f6e6a418ba42de5ba36ddc73e.r2.dev/comprobantes/abc.jpg"
	if got := store.PublicURL("comprobantes/abc.jpg"); got != want {
		t.Fatalf("clave = %q, se esperaba %q", got, want)
	}
	if got := store.PublicURL("https://19e79842547693ffe34fbd1d311d25dc.r2.cloudflarestorage.com/remesas-img/comprobantes/abc.jpg"); got != want {
		t.Fatalf("URL S3 = %q, se esperaba %q", got, want)
	}
}

func TestPublicURLUsaR2PublicDomainComoRespaldo(t *testing.T) {
	t.Setenv("R2_PUBLIC_BASE_URL", "")
	t.Setenv("R2_PUBLIC_DOMAIN", "cdn.example.test")

	store, err := New(Config{
		Endpoint: "https://account.r2.cloudflarestorage.com",
		Bucket:   "remesas-img", AccessKey: "access", SecretKey: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "https://cdn.example.test/comprobantes/abc.jpg"
	if got := store.PublicURL("comprobantes/abc.jpg"); got != want {
		t.Fatalf("url = %q, se esperaba %q", got, want)
	}
}

func TestPublicURLNoExponeEndpointS3(t *testing.T) {
	t.Setenv("R2_PUBLIC_BASE_URL", "")
	t.Setenv("R2_PUBLIC_DOMAIN", "")

	store, err := New(Config{
		Endpoint: "https://account.r2.cloudflarestorage.com",
		Bucket:   "remesas-img", AccessKey: "access", SecretKey: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := store.PublicURL("comprobantes/abc.jpg"); got != "" {
		t.Fatalf("sin base pública no debe construir URL: %q", got)
	}
}
