package postgres

import (
	"strings"
	"testing"
)

func TestCatalogQueriesUseCanonicalViews(t *testing.T) {
	t.Parallel()

	for name, query := range map[string]string{
		"perfil": sqlProfileByGroup,
		"tasa":   sqlLatestRateLot,
	} {
		lower := strings.ToLower(query)
		if strings.Contains(lower, "perfiles_glaukov") || strings.Contains(lower, "tasas_glaukov") {
			t.Fatalf("%s no debe leer tablas físicas: %s", name, query)
		}
	}

	if !strings.Contains(strings.ToLower(sqlProfileByGroup), "from perfil") {
		t.Fatalf("sqlProfileByGroup debe leer FROM perfil: %s", sqlProfileByGroup)
	}
	if !strings.Contains(strings.ToLower(sqlLatestRateLot), "from tasa") {
		t.Fatalf("sqlLatestRateLot debe leer FROM tasa: %s", sqlLatestRateLot)
	}
}
