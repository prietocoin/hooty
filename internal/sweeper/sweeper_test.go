package sweeper

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hooty/internal/clock"
)

func TestSQLDelSweeperNoTocaComprobantes(t *testing.T) {
	t.Parallel()

	for name, query := range map[string]string{
		"procesados": sqlDeleteProcessed,
		"huerfanos":  sqlDeleteOrphans,
		"pares":      sqlDeleteFailedPairs,
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
		"pares":      sqlDeleteFailedPairs,
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

func TestDeleteOrphansSQLProtegeParesViejosYNuevos(t *testing.T) {
	t.Parallel()

	orphans := strings.ToLower(sqlDeleteOrphans)
	if !strings.Contains(orphans, "pareja_id is null") {
		t.Fatalf("sqlDeleteOrphans debe exigir pareja_id IS NULL: %s", sqlDeleteOrphans)
	}
	if strings.Contains(orphans, "pareja_id is not null") {
		t.Fatalf("sqlDeleteOrphans no debe seleccionar pares: %s", sqlDeleteOrphans)
	}

	pairs := strings.ToLower(sqlDeleteFailedPairs)
	for _, fragment := range []string{
		"join impactos p",
		"i.pareja_id is not null",
		"i.procesado = false",
		"p.procesado = false",
		"i.recibido_en < $1",
		"p.recibido_en < $1",
	} {
		if !strings.Contains(pairs, fragment) {
			t.Fatalf("sqlDeleteFailedPairs debe contener %q: %s", fragment, sqlDeleteFailedPairs)
		}
	}
}

func TestSweeperNoBorraParViejoNuevo(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("HOOTY_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("HOOTY_TEST_DATABASE_URL no está definido; test de integración omitido")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("conectar PostgreSQL: %v", err)
	}
	schema := fmt.Sprintf("sweeper_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		admin.Close()
		t.Fatalf("crear esquema de prueba: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
		admin.Close()
		t.Fatalf("parsear DSN: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
		admin.Close()
		t.Fatalf("crear pool de prueba: %v", err)
	}
	defer func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
		admin.Close()
	}()

	if _, err := pool.Exec(ctx, `
CREATE TABLE impactos (
    id BIGSERIAL PRIMARY KEY,
    hash_largo TEXT NOT NULL,
    hash_corto TEXT NOT NULL,
    id_grupo TEXT NOT NULL,
    caption TEXT NOT NULL DEFAULT '',
    instancia TEXT NOT NULL,
    procesado BOOLEAN NOT NULL DEFAULT FALSE,
    recibido_en TIMESTAMP NOT NULL,
    pareja_id BIGINT REFERENCES impactos(id) ON DELETE SET NULL
)`); err != nil {
		t.Fatalf("crear impactos de prueba: %v", err)
	}

	now := time.Date(2026, 10, 26, 12, 0, 0, 0, clock.Caracas())
	insert := func(hash, grupo string, recibido time.Time) int64 {
		t.Helper()
		corto := strings.ToUpper(hash)
		if len(corto) > 8 {
			corto = corto[:8]
		}
		var id int64
		if err := pool.QueryRow(ctx, `
INSERT INTO impactos (hash_largo, hash_corto, id_grupo, caption, instancia, procesado, recibido_en)
VALUES ($1, $2, $3, '', 'hooty', FALSE, $4)
RETURNING id`, hash, corto, grupo, clock.ForTimestamp(recibido)).Scan(&id); err != nil {
			t.Fatalf("insertar impacto: %v", err)
		}
		return id
	}
	emparejar := func(a, b int64) {
		t.Helper()
		if _, err := pool.Exec(ctx, `UPDATE impactos SET pareja_id = $1 WHERE id = $2`, b, a); err != nil {
			t.Fatalf("reservar pareja A: %v", err)
		}
		if _, err := pool.Exec(ctx, `UPDATE impactos SET pareja_id = $1 WHERE id = $2`, a, b); err != nil {
			t.Fatalf("reservar pareja B: %v", err)
		}
	}

	oldA := insert("old-pair", "old-a", now.Add(-48*time.Hour))
	oldB := insert("old-pair", "old-b", now.Add(-47*time.Hour))
	emparejar(oldA, oldB)

	mixedOld := insert("mixed-pair", "mixed-old", now.Add(-48*time.Hour))
	mixedNew := insert("mixed-pair", "mixed-new", now.Add(-1*time.Hour))
	emparejar(mixedOld, mixedNew)

	newA := insert("new-pair", "new-a", now.Add(-2*time.Hour))
	newB := insert("new-pair", "new-b", now.Add(-1*time.Hour))
	emparejar(newA, newB)

	sweeper, err := New(pool, Options{
		Interval:           time.Hour,
		ProcessedRetention: time.Hour,
		OrphanRetention:    24 * time.Hour,
		Clock:              clock.Fixed{T: now},
		Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := sweeper.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.Pares != 2 {
		t.Fatalf("pares purgados = %d, se esperaban 2 (solo el par viejo+viejo)", result.Pares)
	}

	count := func(ids ...int64) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM impactos WHERE id = ANY($1)`, ids).Scan(&n); err != nil {
			t.Fatalf("contar impactos: %v", err)
		}
		return n
	}
	if n := count(oldA, oldB); n != 0 {
		t.Fatalf("el par viejo+viejo debe purgarse, quedan %d filas", n)
	}
	if n := count(mixedOld, mixedNew); n != 2 {
		t.Fatalf("el par viejo+nuevo no debe borrarse, quedan %d filas", n)
	}
	if n := count(newA, newB); n != 2 {
		t.Fatalf("el par nuevo no debe borrarse, quedan %d filas", n)
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
