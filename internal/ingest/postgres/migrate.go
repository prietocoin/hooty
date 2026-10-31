package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureZeroRateAllowed migra chk_tasas_positivas a >= 0. Una tasa 0 es un
// estado contable válido cuando un socio no tiene regla para esa divisa. La
// función es idempotente y solo altera la tabla si la definición vigente aún
// exige tasas estrictamente positivas.
func EnsureZeroRateAllowed(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("postgres: pool es obligatorio para migrar tasas")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: abrir migración de tasas: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Serializa el DDL entre instancias que arranquen a la vez.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('hooty:chk_tasas_positivas'))`); err != nil {
		return fmt.Errorf("postgres: bloquear migración de tasas: %w", err)
	}

	var definition string
	err = tx.QueryRow(ctx, sqlRateCheckDefinition).Scan(&definition)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("postgres: leer restricción de tasas: %w", err)
	}
	if err == nil && rateCheckAllowsZero(definition) {
		return tx.Commit(ctx)
	}

	if _, err := tx.Exec(ctx, sqlDropRateCheck); err != nil {
		return fmt.Errorf("postgres: relajar restricción de tasas: %w", err)
	}
	if _, err := tx.Exec(ctx, sqlAddRateCheck); err != nil {
		return fmt.Errorf("postgres: crear restricción de tasas no negativas: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: confirmar migración de tasas: %w", err)
	}
	return nil
}

// EnsurePlantillaColumn garantiza la columna de plantilla usada por el
// directorio para memorizar/restaurar el estado de los socios.
func EnsurePlantillaColumn(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("postgres: pool es obligatorio para migrar la plantilla")
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE perfiles_glaukov ADD COLUMN IF NOT EXISTS recordar_mostrar jsonb`); err != nil {
		return fmt.Errorf("postgres: crear columna recordar_mostrar: %w", err)
	}
	return nil
}

// EnsureTipoOpColumn garantiza tipo_op con contrato D/P/A y migra filas nulas.
func EnsureTipoOpColumn(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("postgres: pool es obligatorio para migrar tipo_op")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: abrir migración tipo_op: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('hooty:tipo_op'))`); err != nil {
		return fmt.Errorf("postgres: bloquear migración tipo_op: %w", err)
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE comprobantes ADD COLUMN IF NOT EXISTS tipo_op text DEFAULT 'D'`); err != nil {
		return fmt.Errorf("postgres: crear tipo_op: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE comprobantes SET tipo_op = 'D' WHERE tipo_op IS NULL OR UPPER(BTRIM(tipo_op)) NOT IN ('D', 'P', 'A')`); err != nil {
		return fmt.Errorf("postgres: normalizar tipo_op: %w", err)
	}
	if _, err := tx.Exec(ctx, `
DO $$
DECLARE restriccion record;
BEGIN
    FOR restriccion IN
        SELECT c.conname
        FROM pg_constraint c
        JOIN pg_class t ON t.oid = c.conrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        WHERE n.nspname = 'public'
          AND t.relname = 'comprobantes'
          AND c.contype = 'c'
          AND pg_get_constraintdef(c.oid) ILIKE '%tipo_op%'
    LOOP
        EXECUTE format('ALTER TABLE comprobantes DROP CONSTRAINT %I', restriccion.conname);
    END LOOP;
END $$`); err != nil {
		return fmt.Errorf("postgres: limpiar restricciones tipo_op: %w", err)
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE comprobantes DROP CONSTRAINT IF EXISTS chk_tipo_op`); err != nil {
		return fmt.Errorf("postgres: eliminar CHECK tipo_op previo: %w", err)
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE comprobantes ADD CONSTRAINT chk_tipo_op CHECK (tipo_op IN ('D', 'P', 'A'))`); err != nil {
		return fmt.Errorf("postgres: crear CHECK tipo_op: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: confirmar migración tipo_op: %w", err)
	}
	return nil
}

// rateCheckAllowsZero reconoce una definición que ya admite el estado 0.
func rateCheckAllowsZero(definition string) bool {
	lower := strings.ToLower(definition)
	return strings.Contains(lower, "tasa_1 >=") && strings.Contains(lower, "tasa_2 >=")
}
