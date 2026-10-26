package sweeper

// El sweeper solo puede leer y borrar impactos. Estas constantes no deben
// contener nunca la palabra comprobantes: el libro contable es append-only
// para este worker.
const (
	sqlTryLock = `SELECT pg_try_advisory_lock($1)`
	sqlUnlock  = `SELECT pg_advisory_unlock($1)`

	sqlDeleteProcessed = `
DELETE FROM impactos
WHERE procesado = TRUE
  AND recibido_en < $1`

	// Si un lado del par cae por antigüedad, se arrastra al otro para no dejar
	// un pareja_id apuntando a una fila borrada. El UNION elimina duplicados.
	sqlDeleteOrphans = `
WITH victims AS (
    SELECT id
    FROM impactos
    WHERE procesado = FALSE
      AND recibido_en < $1
), ids AS (
    SELECT id FROM victims
    UNION
    SELECT pareja_id
    FROM impactos
    WHERE id IN (SELECT id FROM victims)
      AND pareja_id IS NOT NULL
)
DELETE FROM impactos
WHERE id IN (SELECT id FROM ids)`
)
