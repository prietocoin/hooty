# Reporte de ajustes del pipeline de Hooty v2

## 1. Metadatos

| Campo | Valor |
|---|---|
| Fecha | 2026-10-26 |
| Repositorio | `prietocoin/hooty` |
| Rama | `main` |
| Commit base al iniciar | `8b2c297e5318a365a6a33cb6462574b0d0edcded` |
| Alcance | Corrección DATE-01 y SWEEP-01, tests de regresión y verificación local |
| Push | **No ejecutado** (instrucción expresa) |

Este reporte documenta dos correcciones de regla de negocio y criticidad:

- **DATE-01:** la fecha de operación del binomio se toma del segundo impacto.
- **SWEEP-01:** el sweeper ya no puede borrar un par viejo+nuevo.

---

## 2. DATE-01 — Fecha de operación del segundo impacto

### 2.1 Problema

El pipeline usaba `first.RecibidoEn` para `FechaOperacion`. Eso hacía que la
fecha contable dependiera del primer mensaje del par, no del impacto que cierra
el binomio.

### 2.2 Cambio aplicado

En `internal/ingest/service.go`:

- Línea **209**: el `LiquidacionInput.FechaOperacion` ahora recibe
  `second.RecibidoEn`.
- Líneas **233-235**: se agregó el comentario de negocio y la asignación final
  `comprobante.FechaOperacion = second.RecibidoEn`.

La fecha sigue siendo una hora de pared de `America/Caracas`; no se introduce
UTC ni sufijo `Z`. El cambio solo modifica **qué impacto** define la fecha.

### 2.3 Tests actualizados

En `internal/ingest/service_test.go`:

- Línea **260**: el test del liquidador exige que `FechaOperacion` sea igual al
  segundo impacto (`testNow`), no al primero.
- Línea **270**: se valida también el `Comprobante` guardado
  (`saved.FechaOperacion`).

El test usa dos impactos con fechas distintas:

```text
first  = testNow - 5m
second = testNow
```

Así una regresión que vuelva a usar el primero falla de inmediato.

### 2.4 Veredicto

**[CUMPLE]** DATE-01.

Evidencia:

```text
internal/ingest/service.go:209,233-235
internal/ingest/service_test.go:260,270
```

Riesgo residual: la fecha sigue siendo la recepción del segundo impacto, no la
fecha impresa en el comprobante ni el timestamp de WhatsApp. Es la política
solicitada para esta corrección; la extracción de fecha del recibo queda como
deuda separada.

---

## 3. SWEEP-01 — Protección de pares en el sweeper

### 3.1 Problema

La consulta anterior de huérfanos seleccionaba cualquier impacto no procesado
con `recibido_en` antiguo, incluso si ya tenía `pareja_id`. Después arrastraba
al compañero. Un par formado por un primer impacto de más de 24 h y un segundo
impacto recién llegado podía ser borrado completo.

### 3.2 Cambio aplicado

En `internal/sweeper/sql.go`:

- **`sqlDeleteOrphans` (líneas 17-22):** ahora exige `pareja_id IS NULL`.
  Nunca selecciona impactos emparejados.
- **`sqlDeleteFailedPairs` (líneas 26-38):** nueva consulta para pares
  fallidos. Solo selecciona pares donde:
  - ambos lados tienen `procesado = FALSE`;
  - ambos lados tienen `recibido_en < $1`;
  - el `JOIN impactos p ON p.id = i.pareja_id` confirma que el compañero
    también superó la retención.

En `internal/sweeper/sweeper.go`:

- **Result.Pares (línea 31):** se contabilizan los pares purgados.
- **runAndLog (líneas 153-160):** se registra `pares`.
- **RunOnce (líneas 207-223):** se ejecutan ambas consultas en la misma
  transacción y se devuelven `Huerfanos` y `Pares`.

### 3.3 Tests de regresión

En `internal/sweeper/sweeper_test.go`:

- **`TestDeleteOrphansSQLProtegeParesViejosYNuevos` (línea 51):**
  - exige `pareja_id IS NULL` en `sqlDeleteOrphans`;
  - prohíbe `pareja_id IS NOT NULL` en esa consulta;
  - exige que `sqlDeleteFailedPairs` tenga el `JOIN` y las dos condiciones de
    retención (`i.recibido_en < $1` y `p.recibido_en < $1`), además de
    `procesado = FALSE` en ambos lados.
- **`TestSweeperNoBorraParViejoNuevo` (línea 77):** test de integración
  gated por `HOOTY_TEST_DATABASE_URL`. Crea un esquema temporal y tres pares:
  1. viejo + viejo: debe purgarse;
  2. viejo + nuevo: **no** debe borrarse;
  3. nuevo + nuevo: no debe borrarse.
  Verifica `result.Pares == 2` y que el par viejo+nuevo conserve sus dos filas.

El test de integración se omite localmente si no existe
`HOOTY_TEST_DATABASE_URL`; el test de contrato SQL se ejecuta siempre.

### 3.4 Veredicto

**[CUMPLE]** SWEEP-01.

Evidencia:

```text
internal/sweeper/sql.go:17-38
internal/sweeper/sweeper.go:31,153-160,207-223
internal/sweeper/sweeper_test.go:51,77
```

Riesgo residual: un par con ambos lados de más de 24 h y todavía sin procesar
se considera par fallido y se purga; es la política acordada. Un par
viejo+nuevo queda intacto para permitir que el segundo impacto complete su
procesamiento.

---

## 4. Verificación ejecutada

```text
go test ./...          OK
go test -race ./...    OK
go vet ./...           OK
go build ./...         OK
```

Detalle de los tests nuevos:

```text
TestDeleteOrphansSQLProtegeParesViejosYNuevos   PASS
TestSweeperNoBorraParViejoNuevo                 SKIP
  -> HOOTY_TEST_DATABASE_URL no definido
```

El `SKIP` es intencional: el escenario real requiere PostgreSQL. Para
ejecutarlo:

```powershell
$env:HOOTY_TEST_DATABASE_URL = "postgres://usuario:clave@host:5432/db?sslmode=disable"
go test ./internal/sweeper -run TestSweeperNoBorraParViejoNuevo -v
```

---

## 5. Estado actualizado de los hallazgos

| ID | Estado anterior | Estado actual | Evidencia |
|---|---|---|---|
| DATE-01 | [PARCIAL] | **[CUMPLE]** | `service.go:209,233-235`; `service_test.go:260,270` |
| SWEEP-01 | [PARCIAL] | **[CUMPLE]** | `sql.go:17-38`; `sweeper.go:207-223`; tests de contrato y regresión |

Si se recalcula la matriz previa de 17 reglas con estos dos cambios:

- 13 CUMPLE
- 3 PARCIAL
- 1 NO CUMPLE

**Cumplimiento global recalculado: 85.3%**  
Cálculo: `(13 + 3 × 0.5 + 1 × 0) / 17 = 14.5 / 17 = 0.8529`.

Los pendientes que mantienen el porcentaje por debajo del 100% son,
principalmente:

- DB-01: el liquidador aún lee tablas físicas en lugar de las vistas.
- ATOMIC-01: la reserva del par y el procesamiento no comparten transacción.
- SCHEMA-01: el preflight no valida tipos ni constraints.
- RETRY-01: no hay payload durable de media.
- SEC-01: CORS abierto y sin autenticación para futuras mutaciones.

---

## 6. Diff resumido

```diff
internal/ingest/service.go
- FechaOperacion: first.RecibidoEn,
+ FechaOperacion: second.RecibidoEn,
...
- comprobante.FechaOperacion = first.RecibidoEn
+ // La validez contable la determina el segundo impacto.
+ comprobante.FechaOperacion = second.RecibidoEn

internal/ingest/service_test.go
- if !liq.input.FechaOperacion.Equal(first) { ... }
+ if !liq.input.FechaOperacion.Equal(testNow) { ... }
+ if !saved.FechaOperacion.Equal(testNow) { ... }

internal/sweeper/sql.go
- sqlDeleteOrphans: seleccionaba cualquier impacto viejo + pareja
+ sqlDeleteOrphans: WHERE procesado = FALSE AND pareja_id IS NULL ...
+ sqlDeleteFailedPairs: ambos lados procesado = FALSE y ambos recibido_en < $1

internal/sweeper/sweeper.go
+ Result.Pares
+ tx.Exec(sqlDeleteFailedPairs, orphanCutoff)
```

---

## 7. Conclusión

Las dos correcciones quedan implementadas y verificadas localmente:

- La fecha contable del 2x ahora la define el impacto que cierra el par.
- El sweeper no puede seleccionar impactos con pareja en la purga de huérfanos
  y solo purga pares fallidos cuando ambos lados superaron la retención.

No se ejecutó `git push`. El commit local se realiza únicamente con los cambios
de `internal/` y este reporte, conforme a la instrucción.
