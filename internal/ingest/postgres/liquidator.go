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
// solo se ocupa de leer PostgreSQL.
type Liquidator struct {
	pool      *pgxpool.Pool
	now       clock.Clock
	tasaOrder string
}

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
	return ingest.CalcularLiquidacion(input, perfil1, perfil2, lote, l.now.Now())
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
