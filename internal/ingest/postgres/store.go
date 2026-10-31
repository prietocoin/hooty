// Package postgres implementa los puertos de ingesta sobre PostgreSQL.
//
// Las tablas impactos y comprobantes son el contrato definitivo. El store
// asume que la base ya aplicó:
//   - UNIQUE (hash_largo, id_grupo) en impactos;
//   - pareja_id BIGINT con FK a impactos(id) ON DELETE SET NULL;
//   - PK/UNIQUE (hash_largo) en comprobantes;
//   - CHECK de grupos, socios, tasas e importes no nulos.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hooty/internal/clock"
	"hooty/internal/dbschema"
	"hooty/internal/domain"
)

// ErrPairNotReserved indica que el par ya no estaba en el estado esperado.
var ErrPairNotReserved = errors.New("postgres: el par no está reservado o ya fue procesado")

// DB es la porción del pool que necesita el store. La interfaz permite probar
// la transacción con pgxmock sin abrir PostgreSQL.
type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Store implementa ingest.Store y la verificación del esquema.
type Store struct {
	pool DB
}

// New construye el store con un pool ya validado.
func New(pool DB) (*Store, error) {
	if pool == nil {
		return nil, errors.New("postgres: pool es obligatorio")
	}
	return &Store{pool: pool}, nil
}

// RegistrarImpacto serializa por hash y deja el impacto en la sala de espera.
// La imagen no se toca aquí: el límite transaccional es texto, hash y pareja.
func (s *Store) RegistrarImpacto(ctx context.Context, impacto domain.Impacto) (domain.Registro, error) {
	impacto = domain.NormalizeImpacto(impacto)
	if err := impacto.Validate(); err != nil {
		return domain.Registro{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Registro{}, fmt.Errorf("abrir transacción de impacto: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, sqlAdvisoryLock, "hooty:"+impacto.HashLargo); err != nil {
		return domain.Registro{}, fmt.Errorf("advisory lock por hash: %w", err)
	}

	var inserted domain.Impacto
	err = tx.QueryRow(ctx, sqlInsertImpacto,
		impacto.HashLargo, impacto.HashCorto, impacto.IDGrupo,
		impacto.Caption, impacto.Instancia, clock.ForTimestamp(impacto.RecibidoEn),
	).Scan(
		&inserted.ID, &inserted.HashLargo, &inserted.HashCorto, &inserted.IDGrupo,
		&inserted.Caption, &inserted.Instancia, &inserted.Procesado,
		&inserted.RecibidoEn, &inserted.ParejaID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, selectErr := scanImpacto(tx.QueryRow(ctx, sqlSelectImpacto,
			impacto.HashLargo, impacto.IDGrupo))
		if selectErr != nil {
			return domain.Registro{}, fmt.Errorf("leer impacto existente: %w", selectErr)
		}

		// Recuperación de un par liberado: si el duplicado corresponde al
		// impacto más nuevo (el que cerró el par) y hay una contraparte libre,
		// se vuelve a reservar para reprocesar con la media del evento actual.
		if !existing.Procesado && existing.ParejaID == nil {
			counterpart, findErr := scanImpacto(tx.QueryRow(ctx, sqlFindCounterpart,
				existing.HashLargo, existing.IDGrupo))
			switch {
			case findErr == nil && existing.ID > counterpart.ID:
				if err := reserve(ctx, tx, existing.ID, counterpart.ID); err != nil {
					return domain.Registro{}, err
				}
				if err := reserve(ctx, tx, counterpart.ID, existing.ID); err != nil {
					return domain.Registro{}, err
				}
				existing.ParejaID = &counterpart.ID
				counterpart.ParejaID = &existing.ID
				if err := tx.Commit(ctx); err != nil {
					return domain.Registro{}, fmt.Errorf("commit de reintento de par: %w", err)
				}
				return domain.Registro{
					Status:  domain.StatusPairReady,
					Impacto: existing,
					Pair:    []domain.Impacto{counterpart, existing},
					Conteo:  2,
				}, nil
			case findErr != nil && !errors.Is(findErr, pgx.ErrNoRows):
				return domain.Registro{}, fmt.Errorf("buscar contraparte para reintento: %w", findErr)
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return domain.Registro{}, fmt.Errorf("commit de duplicado: %w", err)
		}
		return domain.Registro{
			Status:  domain.StatusDuplicate,
			Impacto: existing,
			Conteo:  1,
		}, nil
	}
	if err != nil {
		return domain.Registro{}, fmt.Errorf("insertar impacto: %w", err)
	}
	inserted.RecibidoEn = clock.WallClock(inserted.RecibidoEn)

	counterpart, err := scanImpacto(tx.QueryRow(ctx, sqlFindCounterpart,
		impacto.HashLargo, impacto.IDGrupo))
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return domain.Registro{}, fmt.Errorf("commit de espera: %w", err)
		}
		return domain.Registro{
			Status:  domain.StatusWaitingPair,
			Impacto: inserted,
			Conteo:  1,
		}, nil
	}
	if err != nil {
		return domain.Registro{}, fmt.Errorf("buscar contraparte: %w", err)
	}

	if err := reserve(ctx, tx, inserted.ID, counterpart.ID); err != nil {
		return domain.Registro{}, err
	}
	if err := reserve(ctx, tx, counterpart.ID, inserted.ID); err != nil {
		return domain.Registro{}, err
	}

	inserted.ParejaID = &counterpart.ID
	counterpart.ParejaID = &inserted.ID
	// La contraparte ya existía y el impacto recién insertado es el segundo
	// mensaje del binomio. Ese orden es el que usa el servicio para descargar
	// la imagen correcta.
	pair := []domain.Impacto{counterpart, inserted}

	if err := tx.Commit(ctx); err != nil {
		return domain.Registro{}, fmt.Errorf("commit del par: %w", err)
	}
	return domain.Registro{
		Status:  domain.StatusPairReady,
		Impacto: inserted,
		Pair:    pair,
		Conteo:  2,
	}, nil
}

func reserve(ctx context.Context, tx pgx.Tx, id, pareja int64) error {
	tag, err := tx.Exec(ctx, sqlReservePair, id, pareja)
	if err != nil {
		return fmt.Errorf("reservar pareja_id: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrPairNotReserved
	}
	return nil
}

// GuardarComprobanteYMarcar escribe el snapshot con UPSERT y marca los dos
// impactos del par en la misma transacción. El UPSERT es idempotente por
// hash_largo; el UPDATE exige exactamente dos filas.
func (s *Store) GuardarComprobanteYMarcar(ctx context.Context, comprobante domain.Comprobante, ids []int64) error {
	if len(ids) != 2 || ids[0] == ids[1] || ids[0] <= 0 || ids[1] <= 0 {
		return fmt.Errorf("postgres: se requieren dos ids distintos y positivos")
	}
	comprobante = domain.NormalizeComprobante(comprobante)
	if err := comprobante.Validate(); err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("abrir transacción de comprobante: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, sqlUpsertComprobante,
		comprobante.HashLargo, comprobante.HashCorto, comprobante.Instancia,
		comprobante.MontoRecibo, comprobante.MonedaRecibo,
		comprobante.Banco, comprobante.Titular, comprobante.Referencia, comprobante.URLImagen,
		comprobante.Grupo1, comprobante.Grupo2, comprobante.Caption1, comprobante.Caption2,
		comprobante.Socio1, comprobante.Socio2,
		comprobante.Tasa1, comprobante.Tasa2,
		comprobante.Monto1, comprobante.Monto2, comprobante.ME1, comprobante.ME2,
		comprobante.LoteTasa,
		domain.NormalizeTipoOp(comprobante.TipoOp),
		clock.ForTimestamp(comprobante.FechaOperacion),
		clock.ForTimestamp(comprobante.CreadoEn),
		clock.ForTimestamp(comprobante.ActualizadoEn),
	); err != nil {
		return fmt.Errorf("upsert comprobante: %w", err)
	}

	tag, err := tx.Exec(ctx, sqlMarkPairProcessed, ids)
	if err != nil {
		return fmt.Errorf("marcar impactos procesados: %w", err)
	}
	if tag.RowsAffected() != 2 {
		return fmt.Errorf("%w: filas actualizadas=%d", ErrPairNotReserved, tag.RowsAffected())
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit del comprobante: %w", err)
	}
	return nil
}

// ReleasePair limpia la reserva mutua de un par que falló antes del UPSERT.
// No toca filas ya procesadas: si el comprobante se escribió, la reserva debe
// permanecer cerrada.
func (s *Store) ReleasePair(ctx context.Context, ids []int64) error {
	if len(ids) != 2 || ids[0] == ids[1] || ids[0] <= 0 || ids[1] <= 0 {
		return fmt.Errorf("postgres: se requieren dos ids distintos y positivos")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("abrir transacción de liberación: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, sqlReleasePair, ids); err != nil {
		return fmt.Errorf("liberar pareja_id: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit de liberación: %w", err)
	}
	return nil
}

// VerifySchema falla rápido si la base no tiene el contrato que el SQL espera.
func VerifySchema(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("postgres: pool es obligatorio")
	}
	tables := make([]string, 0, len(requiredSchema))
	for table := range requiredSchema {
		tables = append(tables, table)
	}
	rows, err := pool.Query(ctx, sqlSchemaColumns, tables)
	if err != nil {
		return fmt.Errorf("leer information_schema: %w", err)
	}
	defer rows.Close()

	found := map[string]map[string]string{}
	for rows.Next() {
		var table, column, dataType string
		if err := rows.Scan(&table, &column, &dataType); err != nil {
			return fmt.Errorf("leer columna del esquema: %w", err)
		}
		if found[table] == nil {
			found[table] = map[string]string{}
		}
		found[table][column] = dataType
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("recorrer information_schema: %w", err)
	}
	if err := dbschema.Validate(found, requiredSchema); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanImpacto(row rowScanner) (domain.Impacto, error) {
	var impacto domain.Impacto
	var recibidoEn time.Time
	if err := row.Scan(
		&impacto.ID, &impacto.HashLargo, &impacto.HashCorto, &impacto.IDGrupo,
		&impacto.Caption, &impacto.Instancia, &impacto.Procesado,
		&recibidoEn, &impacto.ParejaID,
	); err != nil {
		return domain.Impacto{}, err
	}
	impacto.RecibidoEn = clock.WallClock(recibidoEn)
	return impacto, nil
}
