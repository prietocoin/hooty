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

	// Huérfanos reales: sin pareja asignada. Un impacto emparejado no se toca
	// aquí jamás, aunque su recibido_en sea antiguo.
	sqlDeleteOrphans = `
DELETE FROM impactos
WHERE procesado = FALSE
  AND pareja_id IS NULL
  AND recibido_en < $1`

	// Pares fallidos: solo se purgan cuando AMBOS lados siguen sin procesar y
	// ambos superaron la retención. Un par viejo+nuevo queda intacto para que
	// el segundo impacto pueda completar su procesamiento.
	sqlDeleteFailedPairs = `
WITH victims AS (
    SELECT i.id
    FROM impactos i
    JOIN impactos p ON p.id = i.pareja_id
    WHERE i.procesado = FALSE
      AND i.pareja_id IS NOT NULL
      AND i.recibido_en < $1
      AND p.procesado = FALSE
      AND p.recibido_en < $1
)
DELETE FROM impactos
WHERE id IN (SELECT id FROM victims)`
)
