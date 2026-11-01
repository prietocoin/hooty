package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hooty/internal/clock"
	"hooty/internal/domain"
	"hooty/internal/ingest"
)

// Liquidator resuelve los dos perfiles y el lote vigente a través de las
// vistas canónicas perfil y tasa (ADR-003).
//
// No duplica fórmulas: delega el cálculo puro en ingest.CalcularLiquidacion y
// solo se ocupa de leer y persistir PostgreSQL.
type Liquidator struct {
	pool      *pgxpool.Pool
	now       clock.Clock
	tasaOrder string
	images    ImageResolver
}

// ImageResolver reconstruye o valida la URL pública de una imagen existente.
type ImageResolver interface {
	ResolveImageURL(ctx context.Context, hash, current string) (string, error)
}

// WithImageResolver conecta el resolvedor de R2 al reprocesamiento histórico.
func (l *Liquidator) WithImageResolver(resolver ImageResolver) *Liquidator {
	l.images = resolver
	return l
}

var (
	// ErrManualType impide sobrescribir una edición manual del operador.
	ErrManualType = ingest.ErrManualType
	// ErrReprocessNotFound indica que el hash no existe.
	ErrReprocessNotFound = ingest.ErrComprobanteNotFound
)

// NewLiquidator construye el adaptador financiero y detecta la columna de
// orden de la vista tasa sin asumir que expone `id`.
func NewLiquidator(pool *pgxpool.Pool, now clock.Clock) (*Liquidator, error) {
	if pool == nil {
		return nil, errors.New("postgres: pool es obligatorio para liquidar")
	}
	if now == nil {
		return nil, errors.New("postgres: reloj es obligatorio para liquidar")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	order, err := detectTasaOrder(ctx, pool)
	if err != nil {
		return nil, err
	}
	return &Liquidator{pool: pool, now: now, tasaOrder: order}, nil
}

// Liquidar implementa ingest.Liquidator.
func (l *Liquidator) Liquidar(ctx context.Context, input domain.LiquidacionInput) (domain.Comprobante, error) {
	perfil1, err := l.perfil(ctx, input.Grupo1)
	if err != nil {
		return domain.Comprobante{}, err
	}
	perfil2, err := l.perfil(ctx, input.Grupo2)
	if err != nil {
		return domain.Comprobante{}, err
	}
	lote, err := l.loteVigente(ctx)
	if err != nil {
		return domain.Comprobante{}, err
	}
	fundda, err := l.fundda(ctx)
	if err != nil {
		return domain.Comprobante{}, err
	}
	return ingest.CalcularLiquidacionConFundda(input, perfil1, perfil2, fundda, lote, l.now.Now())
}

// Reprocess recalcula un comprobante histórico no editado manualmente y
// persiste T1/T2, montos, ME y tipo de operación. Acepta hash completo o corto.
func (l *Liquidator) Reprocess(ctx context.Context, hash string) (string, error) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return "", ErrReprocessNotFound
	}

	var (
		hashLargo, hashCorto, instancia, moneda, banco, titular, referencia, urlImagen string
		grupo1, grupo2, caption1, caption2, loteTasa                                   string
		manual                                                                         bool
		monto                                                                          float64
		fecha                                                                          time.Time
	)
	err := l.pool.QueryRow(ctx, `
SELECT hash_largo, COALESCE(hash_corto, ''), COALESCE(instancia, 'hooty'),
       COALESCE(moneda_recibo, ''), COALESCE(monto_recibo, 0),
       COALESCE(banco, ''), COALESCE(titular, ''), COALESCE(referencia, ''),
       COALESCE(url_imagen, ''), COALESCE(grupo_1, ''), COALESCE(grupo_2, ''),
       COALESCE(caption_1, ''), COALESCE(caption_2, ''), COALESCE(lote_tasa, ''),
       COALESCE(tipo_op_manual, FALSE), fecha_operacion
FROM comprobantes
WHERE LOWER(hash_largo) = LOWER($1)
   OR LOWER(hash_largo) LIKE LOWER($1) || '%'
ORDER BY CASE WHEN LOWER(hash_largo) = LOWER($1) THEN 0 ELSE 1 END,
         LENGTH(hash_largo), hash_largo
LIMIT 1`, hash).Scan(
		&hashLargo, &hashCorto, &instancia, &moneda, &monto, &banco, &titular, &referencia, &urlImagen,
		&grupo1, &grupo2, &caption1, &caption2, &loteTasa, &manual, &fecha,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrReprocessNotFound
	}
	if err != nil {
		return "", fmt.Errorf("postgres: leer comprobante a reprocesar: %w", err)
	}
	if manual {
		return "", ErrManualType
	}

	perfil1, err := l.perfil(ctx, grupo1)
	if err != nil {
		return "", err
	}
	perfil2, err := l.perfil(ctx, grupo2)
	if err != nil {
		return "", err
	}
	lote, err := l.lote(ctx, loteTasa)
	if err != nil {
		return "", err
	}
	fundda, err := l.fundda(ctx)
	if err != nil {
		return "", err
	}

	resolvedURL := ""
	if l.images != nil {
		resolvedURL, err = l.images.ResolveImageURL(ctx, hashLargo, urlImagen)
		if err != nil {
			resolvedURL = ""
		}
	}
	calculationURL := resolvedURL
	if strings.TrimSpace(calculationURL) == "" {
		calculationURL = strings.TrimSpace(urlImagen)
	}
	if calculationURL == "" {
		// Comprobante.Validate exige una URL; se usa un marcador temporal y no
		// se persiste porque el UPDATE conserva url_imagen cuando llega vacío.
		calculationURL = "r2://" + hashLargo
	}

	now := l.now.Now()
	comprobante, err := ingest.CalcularLiquidacionConFundda(domain.LiquidacionInput{
		HashLargo:      hashLargo,
		HashCorto:      hashCorto,
		Instancia:      instancia,
		Extraccion:     domain.Extraccion{Monto: monto, Moneda: moneda, Banco: banco, Titular: titular, Referencia: referencia},
		URLImagen:      calculationURL,
		Grupo1:         grupo1,
		Grupo2:         grupo2,
		Caption1:       caption1,
		Caption2:       caption2,
		FechaOperacion: fecha,
	}, perfil1, perfil2, fundda, lote, now)
	if err != nil {
		return "", fmt.Errorf("postgres: recalcular comprobante %s: %w", hashLargo, err)
	}

	tag, err := l.pool.Exec(ctx, `
UPDATE comprobantes
SET tasa_1 = $1, tasa_2 = $2, monto_1 = $3, monto_2 = $4,
    me1 = $5, me2 = $6, tipo_op = $7,
    url_imagen = COALESCE(NULLIF($8, ''), url_imagen),
    actualizado_en = NOW()
WHERE LOWER(hash_largo) = LOWER($9)
  AND COALESCE(tipo_op_manual, FALSE) = FALSE`,
		comprobante.Tasa1, comprobante.Tasa2, comprobante.Monto1, comprobante.Monto2,
		comprobante.ME1, comprobante.ME2, comprobante.TipoOp, resolvedURL, hashLargo)
	if err != nil {
		return "", fmt.Errorf("postgres: guardar reprocesamiento %s: %w", hashLargo, err)
	}
	if tag.RowsAffected() != 1 {
		return "", ErrManualType
	}
	return hashLargo, nil
}

// ReprocessAll reprocesa todas las filas no editadas manualmente.
func (l *Liquidator) ReprocessAll(ctx context.Context) (int, error) {
	rows, err := l.pool.Query(ctx, `
SELECT hash_largo FROM comprobantes
WHERE COALESCE(tipo_op_manual, FALSE) = FALSE
ORDER BY actualizado_en ASC`)
	if err != nil {
		return 0, fmt.Errorf("postgres: listar comprobantes a reprocesar: %w", err)
	}
	var hashes []string
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			rows.Close()
			return 0, fmt.Errorf("postgres: leer hash a reprocesar: %w", err)
		}
		hashes = append(hashes, hash)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("postgres: recorrer comprobantes a reprocesar: %w", err)
	}
	rows.Close()

	count := 0
	var failures []error
	for _, hash := range hashes {
		if _, err := l.Reprocess(ctx, hash); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", hash, err))
			continue
		}
		count++
	}
	return count, errors.Join(failures...)
}

// ReprocessHistoricalOnce ejecuta la regularización completa una sola vez,
// usando el marcador persistido de mantenimiento.
func (l *Liquidator) ReprocessHistoricalOnce(ctx context.Context) (int, error) {
	var done bool
	err := l.pool.QueryRow(ctx, `
SELECT EXISTS (SELECT 1 FROM hooty_maintenance_state WHERE key = 'historical_reprocess_v1')`).Scan(&done)
	if err != nil {
		return 0, fmt.Errorf("postgres: leer estado de reprocesamiento: %w", err)
	}
	if done {
		return 0, nil
	}

	count, err := l.ReprocessAll(ctx)
	if err != nil {
		return count, err
	}
	if _, err := l.pool.Exec(ctx, `
INSERT INTO hooty_maintenance_state (key, value, updated_at)
VALUES ('historical_reprocess_v1', 'done', NOW())
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`); err != nil {
		return count, fmt.Errorf("postgres: marcar reprocesamiento histórico: %w", err)
	}
	return count, nil
}

// ReprocessStale repara al arrancar las filas no editadas que quedaron sin
// imagen, sin tasas o sin liquidar. Es idempotente: cuando la fila ya está
// completa deja de aparecer en la consulta.
func (l *Liquidator) ReprocessStale(ctx context.Context) (int, error) {
	rows, err := l.pool.Query(ctx, `
SELECT hash_largo FROM comprobantes
WHERE COALESCE(tipo_op_manual, FALSE) = FALSE
  AND (
       COALESCE(BTRIM(url_imagen), '') = ''
    OR COALESCE(tasa_1, 0) <= 0
    OR COALESCE(tasa_2, 0) <= 0
    OR COALESCE(monto_1, 0) = 0
    OR COALESCE(monto_2, 0) = 0
    OR COALESCE(me1, 0) = 0
    OR COALESCE(me2, 0) = 0
  )
ORDER BY actualizado_en ASC`)
	if err != nil {
		return 0, fmt.Errorf("postgres: listar comprobantes incompletos: %w", err)
	}
	var hashes []string
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			rows.Close()
			return 0, fmt.Errorf("postgres: leer hash incompleto: %w", err)
		}
		hashes = append(hashes, hash)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("postgres: recorrer comprobantes incompletos: %w", err)
	}
	rows.Close()

	count := 0
	var failures []error
	for _, hash := range hashes {
		if _, err := l.Reprocess(ctx, hash); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", hash, err))
			continue
		}
		count++
	}
	return count, errors.Join(failures...)
}

// Simular ejecuta la liquidación canónica sin persistir el comprobante.
func (l *Liquidator) Simular(ctx context.Context, input ingest.SimulacionInput) (ingest.SimulacionResult, error) {
	perfil1, err := l.perfil(ctx, input.Socio1)
	if err != nil {
		return ingest.SimulacionResult{}, err
	}
	perfil2, err := l.perfil(ctx, input.Socio2)
	if err != nil {
		return ingest.SimulacionResult{}, err
	}
	lote, err := l.lote(ctx, input.LoteTasa)
	if err != nil {
		return ingest.SimulacionResult{}, err
	}
	fundda, err := l.fundda(ctx)
	if err != nil {
		return ingest.SimulacionResult{}, err
	}
	return ingest.SimularLiquidacion(input, perfil1, perfil2, fundda, lote)
}

func (l *Liquidator) lote(ctx context.Context, id string) (domain.LoteTasas, error) {
	if strings.TrimSpace(id) == "" {
		return l.loteVigente(ctx)
	}
	var loteID string
	var raw []byte
	err := l.pool.QueryRow(ctx, `
SELECT COALESCE(id_tasa, ''), COALESCE(tasas::text, '{}')
FROM tasa
WHERE id_tasa = $1
LIMIT 1`, strings.TrimSpace(id)).Scan(&loteID, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return l.loteVigente(ctx)
	}
	if err != nil {
		return domain.LoteTasas{}, fmt.Errorf("postgres: leer lote %q: %w", id, err)
	}
	tasas, err := parseRates(raw)
	if err != nil {
		return domain.LoteTasas{}, err
	}
	if len(tasas) == 0 {
		return domain.LoteTasas{}, domain.Validationf("el lote de tasas %q está vacío", loteID)
	}
	return domain.LoteTasas{ID: strings.TrimSpace(loteID), Tasas: tasas}, nil
}

func (l *Liquidator) perfil(ctx context.Context, grupo string) (domain.Perfil, error) {
	if !domain.IsRealGroup(grupo) {
		return domain.Perfil{}, domain.Validationf("el grupo %q no es un socio real", grupo)
	}
	var perfil domain.Perfil
	var monedas []byte
	err := l.pool.QueryRow(ctx, sqlProfileByGroup, strings.TrimSpace(grupo)).
		Scan(&perfil.Nombre, &perfil.MonedaBase, &monedas, &perfil.Herencia)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Perfil{}, domain.Validationf("el grupo %q no está registrado en la vista perfil", grupo)
	}
	if err != nil {
		return domain.Perfil{}, fmt.Errorf("postgres: leer perfil %q: %w", grupo, err)
	}
	perfil.Nombre = domain.CleanText(perfil.Nombre, 120)
	perfil.MonedaBase = domain.NormalizeCurrency(perfil.MonedaBase, "USDT")
	perfil.Monedas = json.RawMessage(monedas)
	return perfil, nil
}

// fundda devuelve el perfil matriz para resolver la herencia de monedas. Si no
// existe, devuelve un perfil vacío sin romper la simulación.
func (l *Liquidator) fundda(ctx context.Context) (domain.Perfil, error) {
	var perfil domain.Perfil
	var monedas []byte
	err := l.pool.QueryRow(ctx, sqlFunddaProfile).
		Scan(&perfil.Nombre, &perfil.MonedaBase, &monedas, &perfil.Herencia)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Perfil{}, nil
	}
	if err != nil {
		return domain.Perfil{}, fmt.Errorf("postgres: leer perfil FUNDDA: %w", err)
	}
	perfil.Nombre = domain.CleanText(perfil.Nombre, 120)
	perfil.MonedaBase = domain.NormalizeCurrency(perfil.MonedaBase, "USDT")
	perfil.Monedas = json.RawMessage(monedas)
	return perfil, nil
}

func (l *Liquidator) loteVigente(ctx context.Context) (domain.LoteTasas, error) {
	var id string
	var raw []byte
	query := sqlLatestRateLot + " ORDER BY " + l.tasaOrder + " LIMIT 1"
	if err := l.pool.QueryRow(ctx, query).Scan(&id, &raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.LoteTasas{}, domain.Validationf("no hay lote de tasas vigente")
		}
		return domain.LoteTasas{}, fmt.Errorf("postgres: leer lote de tasas: %w", err)
	}
	tasas, err := parseRates(raw)
	if err != nil {
		return domain.LoteTasas{}, err
	}
	if len(tasas) == 0 {
		return domain.LoteTasas{}, domain.Validationf("el lote de tasas %q está vacío", id)
	}
	return domain.LoteTasas{ID: strings.TrimSpace(id), Tasas: tasas}, nil
}

func detectTasaOrder(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	for _, column := range []string{"id", "creado_en", "created_at", "fecha", "id_tasa"} {
		var exists bool
		if err := pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'tasa'
      AND column_name = $1
)`, column).Scan(&exists); err != nil {
			return "", fmt.Errorf("postgres: detectar orden de la vista tasa: %w", err)
		}
		if exists {
			return column + " DESC", nil
		}
	}
	return "id_tasa DESC", nil
}

func parseRates(raw []byte) (map[string]float64, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("postgres: decodificar tasas: %w", err)
	}
	rates := make(map[string]float64, len(decoded))
	for code, value := range decoded {
		rate, ok := numeric(value)
		if !ok || rate <= 0 {
			continue
		}
		rates[domain.NormalizeCurrency(code, "")] = rate
	}
	return rates, nil
}

func numeric(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	case string:
		clean := strings.TrimSpace(strings.ReplaceAll(typed, ",", "."))
		number, err := strconv.ParseFloat(clean, 64)
		return number, err == nil
	default:
		return 0, false
	}
}
