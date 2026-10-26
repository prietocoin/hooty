package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hooty/internal/clock"
	"hooty/internal/domain"
	"hooty/internal/ingest"
)

// Liquidator resuelve los dos perfiles y el lote vigente de tasas_glaukov.
//
// No duplica fórmulas: delega el cálculo puro en ingest.CalcularLiquidacion y
// solo se ocupa de leer PostgreSQL.
type Liquidator struct {
	pool *pgxpool.Pool
	now  clock.Clock
}

// NewLiquidator construye el adaptador financiero.
func NewLiquidator(pool *pgxpool.Pool, now clock.Clock) (*Liquidator, error) {
	if pool == nil {
		return nil, errors.New("postgres: pool es obligatorio para liquidar")
	}
	if now == nil {
		return nil, errors.New("postgres: reloj es obligatorio para liquidar")
	}
	return &Liquidator{pool: pool, now: now}, nil
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

func (l *Liquidator) perfil(ctx context.Context, grupo string) (domain.Perfil, error) {
	if !domain.IsRealGroup(grupo) {
		return domain.Perfil{}, domain.Validationf("el grupo %q no es un socio real", grupo)
	}
	var perfil domain.Perfil
	err := l.pool.QueryRow(ctx, sqlProfileByGroup, strings.TrimSpace(grupo)).
		Scan(&perfil.Nombre, &perfil.MonedaBase)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Perfil{}, domain.Validationf("el grupo %q no está registrado en perfiles_glaukov", grupo)
	}
	if err != nil {
		return domain.Perfil{}, fmt.Errorf("postgres: leer perfil %q: %w", grupo, err)
	}
	perfil.Nombre = domain.CleanText(perfil.Nombre, 120)
	perfil.MonedaBase = domain.NormalizeCurrency(perfil.MonedaBase, "USDT")
	return perfil, nil
}

func (l *Liquidator) loteVigente(ctx context.Context) (domain.LoteTasas, error) {
	var id string
	var raw []byte
	if err := l.pool.QueryRow(ctx, sqlLatestRateLot).Scan(&id, &raw); err != nil {
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
