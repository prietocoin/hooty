package sweeper

import (
	"strings"
	"testing"
	"time"
)

func TestSQLDelSweeperNoTocaComprobantes(t *testing.T) {
	t.Parallel()

	for name, query := range map[string]string{
		"procesados": sqlDeleteProcessed,
		"huerfanos":  sqlDeleteOrphans,
	} {
		if strings.Contains(strings.ToLower(query), "comprobantes") {
			t.Fatalf("%s no puede referenciar la tabla comprobantes", name)
		}
	}
}

func TestSQLDelSweeperSoloBorraImpactos(t *testing.T) {
	t.Parallel()

	for name, query := range map[string]string{
		"procesados": sqlDeleteProcessed,
		"huerfanos":  sqlDeleteOrphans,
	} {
		lower := strings.ToLower(query)
		if !strings.Contains(lower, "delete from impactos") {
			t.Fatalf("%s debe borrar únicamente impactos: %s", name, query)
		}
		if strings.Contains(lower, "update ") || strings.Contains(lower, "insert ") {
			t.Fatalf("%s no debe mutar otras tablas: %s", name, query)
		}
	}
}

func TestNewValidaLaConfiguracion(t *testing.T) {
	t.Parallel()

	valid := Options{
		Interval:           time.Hour,
		ProcessedRetention: time.Hour,
		OrphanRetention:    24 * time.Hour,
	}
	cases := []struct {
		name   string
		mutate func(*Options)
	}{
		{"intervalo", func(o *Options) { o.Interval = 0 }},
		{"procesados", func(o *Options) { o.ProcessedRetention = -1 }},
		{"huerfanos", func(o *Options) { o.OrphanRetention = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := valid
			tc.mutate(&opts)
			if err := validateOptions(opts); err == nil {
				t.Fatal("se esperaba error de configuración")
			}
		})
	}
	if err := validateOptions(valid); err != nil {
		t.Fatalf("configuración válida rechazada: %v", err)
	}
}
