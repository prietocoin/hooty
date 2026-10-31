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

// rateCheckAllowsZero reconoce una definición que ya admite el estado 0.
func rateCheckAllowsZero(definition string) bool {
	lower := strings.ToLower(definition)
	return strings.Contains(lower, "tasa_1 >=") && strings.Contains(lower, "tasa_2 >=")
}
