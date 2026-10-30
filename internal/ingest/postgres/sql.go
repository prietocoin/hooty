package postgres

import "hooty/internal/dbschema"

// El SQL vive en un solo archivo para que el contrato con las tablas
// definitivas sea auditable. Ninguna consulta del store toca comprobantes
// fuera de GuardarComprobanteYMarcar, y el sweeper tiene su propio archivo.

const (
	// Serializa por hash dentro de la transacción: dos grupos simultáneos del
	// mismo comprobante no pueden perder la contraparte.
	sqlAdvisoryLock = `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`

	sqlInsertImpacto = `
INSERT INTO impactos (
    hash_largo, hash_corto, id_grupo, caption, instancia, procesado, recibido_en
)
VALUES ($1, $2, $3, $4, $5, FALSE, $6)
ON CONFLICT (hash_largo, id_grupo) DO NOTHING
RETURNING id, hash_largo, hash_corto, id_grupo, caption, instancia,
          procesado, recibido_en, pareja_id`

	sqlSelectImpacto = `
SELECT id, hash_largo, COALESCE(hash_corto, ''), id_grupo,
       COALESCE(caption, ''), COALESCE(instancia, ''),
       procesado, recibido_en, pareja_id
FROM impactos
WHERE hash_largo = $1 AND id_grupo = $2`

	// Solo una contraparte libre, distinta y no procesada puede cerrar el par.
	sqlFindCounterpart = `
SELECT id, hash_largo, COALESCE(hash_corto, ''), id_grupo,
       COALESCE(caption, ''), COALESCE(instancia, ''),
       procesado, recibido_en, pareja_id
FROM impactos
WHERE hash_largo = $1
  AND id_grupo <> $2
  AND procesado = FALSE
  AND pareja_id IS NULL
ORDER BY recibido_en ASC, id ASC
FOR UPDATE SKIP LOCKED
LIMIT 1`

	sqlReservePair = `
UPDATE impactos
SET pareja_id = $2
WHERE id = $1
  AND pareja_id IS NULL`

	sqlUpsertComprobante = `
INSERT INTO comprobantes (
    hash_largo, hash_corto, instancia,
    monto_recibo, moneda_recibo, banco, titular, referencia, url_imagen,
    grupo_1, grupo_2, caption_1, caption_2,
    socio_1, socio_2, tasa_1, tasa_2, monto_1, monto_2, me1, me2,
    lote_tasa, fecha_operacion, creado_en, actualizado_en
)
VALUES (
    $1, $2, $3,
    $4, $5, $6, $7, $8, $9,
    $10, $11, $12, $13,
    $14, $15, $16, $17, $18, $19, $20, $21,
    $22, $23, $24, $25
)
ON CONFLICT (hash_largo) DO UPDATE SET
    hash_corto = EXCLUDED.hash_corto,
    instancia = EXCLUDED.instancia,
    monto_recibo = EXCLUDED.monto_recibo,
    moneda_recibo = EXCLUDED.moneda_recibo,
    banco = EXCLUDED.banco,
    titular = EXCLUDED.titular,
    referencia = EXCLUDED.referencia,
    url_imagen = EXCLUDED.url_imagen,
    grupo_1 = EXCLUDED.grupo_1,
    grupo_2 = EXCLUDED.grupo_2,
    caption_1 = EXCLUDED.caption_1,
    caption_2 = EXCLUDED.caption_2,
    socio_1 = EXCLUDED.socio_1,
    socio_2 = EXCLUDED.socio_2,
    tasa_1 = EXCLUDED.tasa_1,
    tasa_2 = EXCLUDED.tasa_2,
    monto_1 = EXCLUDED.monto_1,
    monto_2 = EXCLUDED.monto_2,
    me1 = EXCLUDED.me1,
    me2 = EXCLUDED.me2,
    lote_tasa = EXCLUDED.lote_tasa,
    fecha_operacion = EXCLUDED.fecha_operacion,
    actualizado_en = EXCLUDED.actualizado_en`

	// El UPDATE debe afectar exactamente a los dos impactos reservados. Si no,
	// el store hace rollback y deja el par para reintento.
	sqlMarkPairProcessed = `
UPDATE impactos
SET procesado = TRUE
WHERE id = ANY($1)
  AND pareja_id IS NOT NULL
  AND procesado = FALSE`

	// Libera la reserva si el procesamiento externo falla. No toca pares ya
	// procesados; un duplicado del segundo impacto podrá reemparejarlos.
	sqlReleasePair = `
UPDATE impactos
SET pareja_id = NULL
WHERE id = ANY($1)
  AND procesado = FALSE`

	// Lectura de catálogos a través de las vistas canónicas de ADR-003. Las
	// tablas físicas perfiles_glaukov y tasas_glaukov quedan reservadas para
	// escritura.
	sqlLatestRateLot = `
SELECT COALESCE(id_tasa, ''), COALESCE(tasas::text, '{}')
FROM tasa`

	sqlProfileByGroup = `
SELECT COALESCE(nombre, ''), COALESCE(moneda_base, 'USDT'), COALESCE(monedas::text, '{}'),
       COALESCE(herencia, FALSE)
FROM perfil
WHERE LOWER(BTRIM(COALESCE(id_grupo, ''))) = LOWER(BTRIM($1))
   OR LOWER(BTRIM(COALESCE(nombre, ''))) = LOWER(BTRIM($1))
ORDER BY
    (LOWER(BTRIM(COALESCE(id_grupo, ''))) = LOWER(BTRIM($1))) DESC,
    nombre ASC
LIMIT 1`

	sqlFunddaProfile = `
SELECT COALESCE(nombre, ''), COALESCE(moneda_base, 'USDT'), COALESCE(monedas::text, '{}'),
       COALESCE(herencia, FALSE)
FROM perfil
WHERE UPPER(BTRIM(COALESCE(nombre, ''))) = 'FUNDDA'
ORDER BY nombre ASC
LIMIT 1`

	sqlSchemaColumns = `
SELECT table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = ANY($1)
ORDER BY table_name, ordinal_position`

	// El estado contable admite tasa 0 cuando el socio no tiene regla para la
	// divisa. La migración de arranque solo actúa si la restricción aún exige
	// valores estrictamente positivos.
	sqlRateCheckDefinition = `
SELECT COALESCE(pg_get_constraintdef(c.oid), '')
FROM pg_constraint c
JOIN pg_class t ON t.oid = c.conrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
WHERE c.conname = 'chk_tasas_positivas'
  AND t.relname = 'comprobantes'
  AND n.nspname = 'public'
LIMIT 1`

	sqlDropRateCheck = `ALTER TABLE comprobantes DROP CONSTRAINT IF EXISTS chk_tasas_positivas`

	sqlAddRateCheck = `ALTER TABLE comprobantes ADD CONSTRAINT chk_tasas_positivas CHECK (tasa_1 >= 0 AND tasa_2 >= 0)`
)

var (
	columnText      = dbschema.Column{Kind: dbschema.Text}
	columnInteger   = dbschema.Column{Kind: dbschema.Integer}
	columnBoolean   = dbschema.Column{Kind: dbschema.Boolean}
	columnNumeric   = dbschema.Column{Kind: dbschema.Numeric}
	columnTimestamp = dbschema.Column{Kind: dbschema.TimestampWall}
)

var requiredSchema = dbschema.Schema{
	"impactos": {
		"id":          columnInteger,
		"hash_largo":  columnText,
		"hash_corto":  columnText,
		"id_grupo":    columnText,
		"caption":     columnText,
		"instancia":   columnText,
		"procesado":   columnBoolean,
		"recibido_en": columnTimestamp,
		"pareja_id":   columnInteger,
	},
	"comprobantes": {
		"hash_largo":      columnText,
		"hash_corto":      columnText,
		"instancia":       columnText,
		"monto_recibo":    columnNumeric,
		"moneda_recibo":   columnText,
		"banco":           columnText,
		"titular":         columnText,
		"referencia":      columnText,
		"url_imagen":      columnText,
		"grupo_1":         columnText,
		"grupo_2":         columnText,
		"caption_1":       columnText,
		"caption_2":       columnText,
		"socio_1":         columnText,
		"socio_2":         columnText,
		"tasa_1":          columnNumeric,
		"tasa_2":          columnNumeric,
		"monto_1":         columnNumeric,
		"monto_2":         columnNumeric,
		"me1":             columnNumeric,
		"me2":             columnNumeric,
		"lote_tasa":       columnText,
		"fecha_operacion": columnTimestamp,
		"creado_en":       columnTimestamp,
		"actualizado_en":  columnTimestamp,
	},
}
