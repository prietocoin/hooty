package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hooty/internal/clock"
	"hooty/internal/dbschema"
)

// ErrNotFound indica que la fila solicitada no existe.
var ErrNotFound = errors.New("api: registro no encontrado")

// Filter es la consulta normalizada del listado.
type Filter struct {
	Socio       string
	Rol         string
	FechaInicio string
	FechaFin    string
	Hash        string
	Orden       string
	Limit       int
	Offset      int
}

// Repository implementa las lecturas sobre comprobantes, perfil y tasa.
type Repository struct {
	pool      *pgxpool.Pool
	tasaOrder string
}

// NewRepository construye el repositorio y detecta una columna segura para
// ordenar la vista tasa. No asume que la vista exponga `id`.
func NewRepository(ctx context.Context, pool *pgxpool.Pool) (*Repository, error) {
	if pool == nil {
		return nil, errors.New("api: pool es obligatorio")
	}
	order, err := detectTasaOrder(ctx, pool)
	if err != nil {
		return nil, err
	}
	return &Repository{pool: pool, tasaOrder: order}, nil
}

var requiredViewSchema = dbschema.Schema{
	"perfil": {
		"nombre":        {Kind: dbschema.Text},
		"id_grupo":      {Kind: dbschema.Text},
		"rol":           {Kind: dbschema.Text},
		"moneda_base":   {Kind: dbschema.Text},
		"saldo_inicial": {Kind: dbschema.Numeric},
		"mostrar":       {Kind: dbschema.JSON},
		"monedas":       {Kind: dbschema.JSON},
		"herencia":      {Kind: dbschema.Boolean},
	},
	"tasa": {
		"id_tasa": {Kind: dbschema.Text},
		"tasas":   {Kind: dbschema.JSON},
	},
}

// VerifyViews falla rápido si las vistas no tienen las columnas y tipos del
// contrato.
func VerifyViews(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("api: pool es obligatorio")
	}
	tables := make([]string, 0, len(requiredViewSchema))
	for table := range requiredViewSchema {
		tables = append(tables, table)
	}
	rows, err := pool.Query(ctx, `
SELECT table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = ANY($1::text[])`, tables)
	if err != nil {
		return fmt.Errorf("api: leer columnas de vistas: %w", err)
	}
	defer rows.Close()

	found := map[string]map[string]string{}
	for rows.Next() {
		var table, column, dataType string
		if err := rows.Scan(&table, &column, &dataType); err != nil {
			return fmt.Errorf("api: escanear columna de vista: %w", err)
		}
		if found[table] == nil {
			found[table] = map[string]string{}
		}
		found[table][column] = dataType
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("api: recorrer columnas de vistas: %w", err)
	}
	if err := dbschema.Validate(found, requiredViewSchema); err != nil {
		return fmt.Errorf("api: %w", err)
	}
	return nil
}

// ListComprobantes devuelve una página y el total del universo filtrado.
func (r *Repository) ListComprobantes(ctx context.Context, filter Filter) ([]ComprobanteRow, int, error) {
	where, args, err := r.buildWhere(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if where == "WHERE 1=0" {
		return []ComprobanteRow{}, 0, nil
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM comprobantes c `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("api: contar comprobantes: %w", err)
	}
	if total == 0 {
		return []ComprobanteRow{}, 0, nil
	}

	limit := normalizeLimit(filter.Limit)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	dataArgs := append(append([]any(nil), args...), limit, offset)
	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	query := `SELECT ` + sqlComprobanteColumns + ` FROM comprobantes c ` + where +
		` ORDER BY ` + orderClause(filter.Orden) +
		fmt.Sprintf(` LIMIT $%d OFFSET $%d`, limitArg, offsetArg)

	rows, err := r.pool.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("api: listar comprobantes: %w", err)
	}
	defer rows.Close()

	items := make([]ComprobanteRow, 0, limit)
	for rows.Next() {
		item, err := scanComprobante(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("api: recorrer comprobantes: %w", err)
	}
	return items, total, nil
}

// GetComprobante busca una fila por hash largo exacto.
func (r *Repository) GetComprobante(ctx context.Context, hash string) (ComprobanteRow, error) {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if hash == "" {
		return ComprobanteRow{}, ErrNotFound
	}
	row := r.pool.QueryRow(ctx,
		`SELECT `+sqlComprobanteColumns+` FROM comprobantes c WHERE c.hash_largo = $1`, hash)
	item, err := scanComprobante(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return ComprobanteRow{}, ErrNotFound
	}
	if err != nil {
		return ComprobanteRow{}, fmt.Errorf("api: leer comprobante: %w", err)
	}
	return item, nil
}

// ListPerfiles devuelve la vista perfil completa.
func (r *Repository) ListPerfiles(ctx context.Context) ([]Perfil, error) {
	rows, err := r.pool.Query(ctx, `
SELECT
    COALESCE(nombre, ''),
    COALESCE(id_grupo, ''),
    COALESCE(rol, 'SOCIO'),
    COALESCE(moneda_base, 'USDT'),
    COALESCE(saldo_inicial, 0),
    mostrar,
    monedas,
    COALESCE(herencia, FALSE)
FROM perfil
ORDER BY nombre`)
	if err != nil {
		return nil, fmt.Errorf("api: listar perfiles: %w", err)
	}
	defer rows.Close()

	items := make([]Perfil, 0, 64)
	for rows.Next() {
		var p Perfil
		var mostrar, monedas []byte
		if err := rows.Scan(
			&p.Nombre, &p.IDGrupo, &p.Rol, &p.MonedaBase,
			&p.SaldoInicial, &mostrar, &monedas, &p.Herencia,
		); err != nil {
			return nil, fmt.Errorf("api: escanear perfil: %w", err)
		}
		p.Mostrar = json.RawMessage(mostrar)
		p.Monedas = json.RawMessage(monedas)
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("api: recorrer perfiles: %w", err)
	}
	return items, nil
}

// LatestTasa devuelve el último registro de la vista tasa.
func (r *Repository) LatestTasa(ctx context.Context) (Tasa, error) {
	var item Tasa
	var tasas []byte
	query := `SELECT COALESCE(id_tasa, ''), tasas FROM tasa ORDER BY ` + r.tasaOrder + ` LIMIT 1`
	err := r.pool.QueryRow(ctx, query).Scan(&item.IDTasa, &tasas)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tasa{}, ErrNotFound
	}
	if err != nil {
		return Tasa{}, fmt.Errorf("api: leer tasa vigente: %w", err)
	}
	item.Tasas = json.RawMessage(tasas)
	return item, nil
}

// ReportFilters reúne roles, entidades y hashes para el constructor de reportes.
func (r *Repository) ReportFilters(ctx context.Context) (ReportFilters, error) {
	var out ReportFilters

	rows, err := r.pool.Query(ctx, `
SELECT DISTINCT COALESCE(rol, '')
FROM perfil
WHERE COALESCE(rol, '') <> ''
ORDER BY 1`)
	if err != nil {
		return out, fmt.Errorf("api: listar roles: %w", err)
	}
	for rows.Next() {
		var rol string
		if err := rows.Scan(&rol); err != nil {
			rows.Close()
			return out, fmt.Errorf("api: escanear rol: %w", err)
		}
		out.Roles = append(out.Roles, rol)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("api: recorrer roles: %w", err)
	}

	rows, err = r.pool.Query(ctx, `
SELECT nombre
FROM perfil
WHERE COALESCE(nombre, '') <> ''
ORDER BY nombre`)
	if err != nil {
		return out, fmt.Errorf("api: listar entidades: %w", err)
	}
	for rows.Next() {
		var nombre string
		if err := rows.Scan(&nombre); err != nil {
			rows.Close()
			return out, fmt.Errorf("api: escanear entidad: %w", err)
		}
		out.Entidades = append(out.Entidades, nombre)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("api: recorrer entidades: %w", err)
	}

	rows, err = r.pool.Query(ctx, `
SELECT hash_largo, COALESCE(hash_corto, '')
FROM comprobantes
ORDER BY fecha_operacion DESC, hash_largo DESC
LIMIT 2000`)
	if err != nil {
		return out, fmt.Errorf("api: listar hashes: %w", err)
	}
	for rows.Next() {
		var hash, corto string
		if err := rows.Scan(&hash, &corto); err != nil {
			rows.Close()
			return out, fmt.Errorf("api: escanear hash: %w", err)
		}
		label := strings.ToUpper(strings.TrimSpace(corto))
		if label == "" {
			label = strings.ToUpper(hash[:min(8, len(hash))])
		}
		out.Hashes = append(out.Hashes, HashOption{Value: hash, Label: label})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("api: recorrer hashes: %w", err)
	}
	return out, nil
}

const sqlComprobanteColumns = `
    c.hash_largo,
    COALESCE(c.hash_corto, ''),
    COALESCE(c.instancia, ''),
    COALESCE(c.monto_recibo, 0),
    COALESCE(c.moneda_recibo, ''),
    COALESCE(c.banco, ''),
    COALESCE(c.titular, ''),
    COALESCE(c.referencia, ''),
    COALESCE(c.url_imagen, ''),
    COALESCE(c.grupo_1, ''),
    COALESCE(c.grupo_2, ''),
    COALESCE(c.caption_1, ''),
    COALESCE(c.caption_2, ''),
    COALESCE(c.socio_1, ''),
    COALESCE(c.socio_2, ''),
    COALESCE(c.tasa_1, 0),
    COALESCE(c.tasa_2, 0),
    COALESCE(c.monto_1, 0),
    COALESCE(c.monto_2, 0),
    COALESCE(c.me1, 0),
    COALESCE(c.me2, 0),
    COALESCE(c.lote_tasa, ''),
    c.fecha_operacion,
    c.creado_en,
    c.actualizado_en`

func (r *Repository) buildWhere(ctx context.Context, filter Filter) (string, []any, error) {
	conditions := []string{"1=1"}
	args := make([]any, 0, 6)
	add := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	if socio := strings.TrimSpace(filter.Socio); socio != "" {
		arg := add(socio)
		conditions = append(conditions,
			`(LOWER(BTRIM(c.socio_1)) = LOWER(BTRIM(`+arg+`)) OR LOWER(BTRIM(c.socio_2)) = LOWER(BTRIM(`+arg+`)))`)
	}
	if rol := strings.TrimSpace(filter.Rol); rol != "" {
		names, err := r.profileNamesByRole(ctx, rol)
		if err != nil {
			return "", nil, err
		}
		if len(names) == 0 {
			return "WHERE 1=0", nil, nil
		}
		arg := add(names)
		conditions = append(conditions,
			`(LOWER(BTRIM(c.socio_1)) = ANY(`+arg+`::text[]) OR LOWER(BTRIM(c.socio_2)) = ANY(`+arg+`::text[]))`)
	}
	if inicio := strings.TrimSpace(filter.FechaInicio); inicio != "" {
		arg := add(inicio)
		conditions = append(conditions, `c.fecha_operacion >= `+arg+`::date`)
	}
	if fin := strings.TrimSpace(filter.FechaFin); fin != "" {
		arg := add(fin)
		conditions = append(conditions, `c.fecha_operacion < (`+arg+`::date + INTERVAL '1 day')`)
	}
	if hash := strings.ToLower(strings.TrimSpace(filter.Hash)); hash != "" {
		arg := add(hash + "%")
		conditions = append(conditions,
			`(LOWER(c.hash_largo) LIKE `+arg+` OR LOWER(c.hash_corto) LIKE `+arg+`)`)
	}
	return "WHERE " + strings.Join(conditions, " AND "), args, nil
}

func (r *Repository) profileNamesByRole(ctx context.Context, rol string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
SELECT LOWER(BTRIM(nombre))
FROM perfil
WHERE LOWER(BTRIM(COALESCE(rol, ''))) = LOWER(BTRIM($1))
  AND COALESCE(nombre, '') <> ''`, rol)
	if err != nil {
		return nil, fmt.Errorf("api: resolver socios por rol: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("api: escanear socio por rol: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("api: recorrer socios por rol: %w", err)
	}
	return names, nil
}

func scanComprobante(row interface{ Scan(...any) error }) (ComprobanteRow, error) {
	var item ComprobanteRow
	var fecha, creado, actualizado time.Time
	if err := row.Scan(
		&item.HashLargo, &item.HashCorto, &item.Instancia,
		&item.MontoRecibo, &item.MonedaRecibo,
		&item.Banco, &item.Titular, &item.Referencia, &item.URLImagen,
		&item.Grupo1, &item.Grupo2, &item.Caption1, &item.Caption2,
		&item.Socio1, &item.Socio2,
		&item.Tasa1, &item.Tasa2, &item.Monto1, &item.Monto2, &item.ME1, &item.ME2,
		&item.LoteTasa, &fecha, &creado, &actualizado,
	); err != nil {
		return ComprobanteRow{}, err
	}
	item.FechaOperacion = clock.WallClock(fecha)
	item.CreadoEn = clock.WallClock(creado)
	item.ActualizadoEn = clock.WallClock(actualizado)
	return item, nil
}

func orderClause(orden string) string {
	switch strings.ToLower(strings.TrimSpace(orden)) {
	case "fecha_asc":
		return "c.fecha_operacion ASC, c.hash_largo ASC"
	case "monto_asc":
		return "c.monto_recibo ASC, c.fecha_operacion DESC"
	case "monto_desc":
		return "c.monto_recibo DESC, c.fecha_operacion DESC"
	case "fecha_desc":
		fallthrough
	default:
		return "c.fecha_operacion DESC, c.hash_largo DESC"
	}
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 200 {
		return 200
	}
	return limit
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
			return "", fmt.Errorf("api: detectar orden de tasa: %w", err)
		}
		if exists {
			return column + " DESC", nil
		}
	}
	return "id_tasa DESC", nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
