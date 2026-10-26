# ADR 002 — Hooty v2: ingesta atómica 2x con lazy download

- **Estado:** aceptado e implementado
- **Fecha:** 2026-10-25
- **Proyecto:** Hooty v2 (`C:\Proyectos\hooty`)
- **Reemplaza:** el pipeline legacy `internal/ingesta` de Glauk
- **Alcance:** webhook de Evolution, sala de espera, liquidación 2x, UPSERT
  atómico y sweeper de impactos

---

## 1. Contexto

Hooty nace como repositorio independiente para corregir los defectos
estructurales del flujo anterior:

1. Dos mensajes del mismo grupo podían contarse como binomio 2x, produciendo
   `socio_1 == socio_2`.
2. Los grupos no resueltos terminaban en `GENERAL`/`NO DEFINIDO` y se
   liquidaban con montos `0.00`.
3. La imagen se descargaba antes de saber si existía una contraparte real,
   gastando ancho de banda, R2 y llamadas a IA en mensajes 1x.
4. La escritura del comprobante y el cierre de los impactos no eran una sola
   unidad transaccional.
5. El sweeper podía borrar filas válidas y no tenía lock entre réplicas.
6. Las fechas dependían de la zona horaria del proceso o del servidor.

La base PostgreSQL ya contiene las tablas definitivas `impactos` y
`comprobantes`, con sellado de hora a `America/Caracas` y CHECK de integridad
contable. Este ADR documenta cómo el código respeta ese contrato.

### Objetivos

- Persistir el primer impacto **sin descargar ni subir la imagen**.
- Procesar únicamente el segundo impacto de un **grupo distinto**.
- Validar el binomio antes de llamar a IA o R2.
- Escribir `comprobantes` con UPSERT y marcar exactamente los dos impactos en
  la misma transacción.
- Purgar procesados (>1 h) y huérfanos (>24 h) con un lock de sesión.
- No tocar nunca `comprobantes` desde el sweeper.

### No objetivos de esta fase

- Cola durable de reintentos con payload de media.
- Migración de `float64` a `decimal`/enteros de menor unidad.
- Polaridad, spread y herencia completas del motor financiero histórico.
- Métricas Prometheus/OpenTelemetry.
- Reescritura del frontend.

---

## 2. Decisiones

### D1 — Repositorio y módulo independientes

`module hooty`, Go 1.27, sin dependencias de UI ni del árbol de Glauk. El
único módulo externo de infraestructura es `github.com/jackc/pgx/v5`; HTTP y
logging usan la biblioteca estándar (`net/http`, `log/slog`).

La estructura queda:

```text
cmd/server/main.go
internal/clock/
internal/domain/
internal/ingest/
internal/ingest/evolution/
internal/ingest/gemini/
internal/ingest/postgres/
internal/ingest/r2/
internal/sweeper/
docs/adr/002-hooty-ingestion-v2.md
```

### D2 — Hora oficial de Venezuela

`internal/clock` es la única fuente de tiempo operativo:

- `Caracas()` carga `America/Caracas` y, si falla, devuelve
  `time.FixedZone("VET", -4h)`.
- Se importa `_ "time/tzdata"` para que la zona exista en imágenes mínimas.
- `System.Now()` siempre devuelve la hora en Caracas.
- `FromUnix` acepta segundos, milisegundos, microsegundos y nanosegundos.
- `ForTimestamp` expresa la hora de pared en UTC antes de escribir una columna
  `timestamp without time zone`; pgx descarta la zona y conserva los
  componentes, evitando dobles conversiones.
- `WallClock` adjunta Caracas a una hora leída de una columna `timestamp` sin
  mover sus componentes.

Convención asumida y reforzada por el pool (`timezone=America/Caracas`):

```text
impactos.recibido_en       timestamp without time zone, hora de pared Caracas
comprobantes.fecha_operacion timestamp without time zone, hora de pared Caracas
comprobantes.creado_en     timestamp without time zone, hora de pared Caracas
comprobantes.actualizado_en timestamp without time zone, hora de pared Caracas
```

Si la base usa `timestamptz`, los CHECK y el sweeper siguen funcionando con
instantes absolutos, pero debe revisarse la convención de lectura antes de
desplegar. El preflight de esquema no puede distinguir ambos tipos por sí solo.

### D3 — El dominio es la primera defensa

`internal/domain` define `Impacto`, `Comprobante`, `Extraccion`, `Perfil`,
`LoteTasas` y las validaciones compartidas. Las reglas mínimas son:

- `hash_largo` y `hash_corto` obligatorios.
- `id_grupo`, `grupo_1` y `grupo_2` reales, no placeholders
  (`GENERAL`, `NO DEFINIDO`, `S/D`, `N/A`, `NULL`, `UNDEFINED`, `-`, vacío).
- Los dos grupos deben ser distintos tras normalizar mayúsculas, espacios y
  diacríticos.
- `socio_1` y `socio_2` reales y distintos.
- `monto_recibo > 0`; `tasa_1 > 0`, `tasa_2 > 0`.
- `abs(monto_1) >= 0.01`, `abs(monto_2) >= 0.01`.
- `abs(me1) >= 0.01`, `abs(me2) >= 0.01`.
- `lote_tasa`, `fecha_operacion`, `creado_en` y `url_imagen` obligatorios.
- Ningún valor monetario puede ser `NaN` o `Inf`.

La base repite estas reglas con CHECK; si el dominio y el CHECK discrepan, el
insert falla y la transacción hace rollback.

### D4 — Sala de espera con reserva de par

`Store.RegistrarImpacto` abre una transacción y toma un advisory lock por
hash:

```sql
SELECT pg_advisory_xact_lock(hashtextextended($1, 0));
```

Esto serializa dos grupos simultáneos del mismo comprobante. Después inserta
con `ON CONFLICT (hash_largo, id_grupo) DO NOTHING`. La unicidad de esa pareja
es obligatoria para que dos entregas del mismo grupo no dupliquen la sala de
espera.

Si el impacto ya existía, se devuelve `duplicate` y **no** se descarga nada.
Si es nuevo, se busca una contraparte libre:

```sql
SELECT ...
FROM impactos
WHERE hash_largo = $1
  AND id_grupo <> $2
  AND procesado = FALSE
  AND pareja_id IS NULL
ORDER BY recibido_en ASC, id ASC
FOR UPDATE SKIP LOCKED
LIMIT 1;
```

Si no hay contraparte, el impacto queda con `procesado=false` y
`pareja_id=NULL`; la respuesta es `waiting_pair`. La imagen no se toca.

Si hay contraparte, se reservan ambos lados con actualizaciones condicionales
(`pareja_id IS NULL`) y se devuelve `pair_ready`. Un tercer grupo encontrará
`pareja_id` ocupado y esperará, sin romper un par fallido.

### D5 — Lazy download

El servicio solo llama a `MediaFetcher.Descargar` cuando el store devuelve
`pair_ready`, y usa la `key`/`message` del evento actual, que es el segundo
impacto. El primer impacto nunca genera tráfico de media.

Secuencia del segundo impacto:

```text
Descargar (una sola vez)
  -> Subir a R2 con clave comprobantes/{hash}.jpg
  -> Gemini con el caption del segundo impacto
  -> Validar extracción (monto > 0, moneda)
  -> Resolver perfiles y lote de tasas
  -> Calcular snapshot 2x
  -> Validar Comprobante
  -> Transacción: UPSERT comprobantes + UPDATE dos impactos
```

La clave R2 es determinista para que un reintento sobrescriba el mismo objeto.

### D6 — Transacción final y UPSERT

`GuardarComprobanteYMarcar` usa una única transacción:

1. `INSERT ... ON CONFLICT (hash_largo) DO UPDATE` sobre `comprobantes`.
2. `UPDATE impactos SET procesado = TRUE WHERE id = ANY($1) ...`.
3. Commit solo si el UPDATE afectó **exactamente dos filas**.

El UPDATE exige `pareja_id IS NOT NULL` y `procesado = FALSE`. Si afecta a
cero, una o tres filas, se devuelve `ErrPairNotReserved` y la transacción hace
rollback: no queda un comprobante sin sus dos impactos cerrados.

`creado_en` se conserva en el conflicto; `actualizado_en` se reemplaza con el
nuevo snapshot.

### D7 — Sweeper

`internal/sweeper` arranca un ticker de una hora y ejecuta una pasada al
inicio. Usa una conexión dedicada y un lock de sesión:

```sql
SELECT pg_try_advisory_lock(0x484F4F54595357); -- "HOOTYSW"
```

Si otra réplica tiene el lock, la pasada se omite sin error. La liberación se
hace en la misma conexión.

Los cortes se calculan en Go desde el reloj de Caracas y se envían como
parámetros, evitando depender de la zona de sesión:

```sql
DELETE FROM impactos
WHERE procesado = TRUE
  AND recibido_en < $1;  -- now - 1h

WITH victims AS (
    SELECT id FROM impactos
    WHERE procesado = FALSE
      AND recibido_en < $1  -- now - 24h
), ids AS (
    SELECT id FROM victims
    UNION
    SELECT pareja_id FROM impactos
    WHERE id IN (SELECT id FROM victims)
      AND pareja_id IS NOT NULL
)
DELETE FROM impactos WHERE id IN (SELECT id FROM ids);
```

El borrado de huérfanos arrastra al compañero del par para no dejar un
`pareja_id` colgante. El sweeper no referencia `comprobantes` en ninguna
consulta; un test unitario falla si esa palabra aparece en su SQL.

### D8 — Adaptadores

- `evolution.Client`: descarga base64 desde `getBase64FromMediaMessage`,
  probando las rutas `message` y `chat`.
- `gemini.Client`: rota claves/modelos, exige JSON, normaliza montos con coma
  decimal y devuelve solo el contrato de cinco campos.
- `r2.Store`: PUT S3 con firma SigV4 manual, sin SDK; URL pública determinista.
- `postgres.Store`: sala de espera, UPSERT y marcado atómico.
- `postgres.Liquidator`: lee `perfiles_glaukov` y el último lote de
  `tasas_glaukov`, y delega el cálculo puro.
- `ingest.HTTPHandler`: parsea Evolution, autentica opcionalmente y traduce
  errores a 200/400/422/500.

### D9 — Liquidación base

`ingest.CalcularLiquidacion` implementa la fórmula inicial:

```text
tasa_lado = tasa(moneda_recibo) / tasa(moneda_base_socio)
monto_lado = monto_recibo / tasa_lado
me_lado = monto_lado / tasa(moneda_base_socio)
```

Las tasas se leen del lote vigente (`tasas_glaukov` ordenado por `id DESC`).
`USD`/`USDT` valen 1 si el lote no las trae. Si cualquiera de los dos lados se
trunca por debajo de 0.01, la liquidación se rechaza; no se persiste un
`0.00`.

Polaridad, spread, herencia y reglas de abono quedan como deuda P1. El
contrato de columnas ya está congelado y los CHECK impiden snapshots vacíos.

### D10 — Seguridad y observabilidad

- Token opcional por `X-Hooty-Token` o `Authorization: Bearer`.
- Comparación en tiempo constante.
- `Cache-Control: no-store` en respuestas.
- Límite de body y de imagen.
- Logs estructurados sin captions ni binarios.
- `/healthz` hace ping a PostgreSQL.
- `VerifySchema` falla al arrancar si faltan columnas del contrato.

---

## 3. Flujo completo

```text
Evolution POST /webhooks/evolution
        |
        v
ParseEvento (hash, grupo, caption, key/message crudos)
        |
        v
Service.Recibir
        |
        +--> Store.RegistrarImpacto (tx + advisory lock por hash)
        |        |
        |        +-- mismo (hash, grupo) --> duplicate --> 200, 0 bytes
        |        |
        |        +-- sin contraparte --> waiting_pair --> 200, 0 bytes
        |        |
        |        +-- contraparte libre --> reserva pareja_id mutuo
        |                                  --> pair_ready
        |
        +--> process (solo pair_ready)
                 |
                 +-- Descargar SOLO la imagen del segundo impacto
                 +-- Subir a R2 (clave determinista)
                 +-- Gemini + caption del segundo
                 +-- Validar monto/moneda
                 +-- Perfiles + lote de tasas + cálculo 2x
                 +-- Validar Comprobante
                 +-- TX: UPSERT comprobantes
                 |       UPDATE impactos ... exactamente 2
                 +-- COMMIT
                 |
                 v
              200 processed
```

### Matriz de fallos

| Momento | Estado resultante | Recuperación |
|---|---|---|
| Falla antes del INSERT | Nada persistido | El webhook puede reintentar |
| Primer impacto, sin par | `procesado=false`, `pareja_id=NULL` | Espera 24 h; el sweeper purga |
| Segundo impacto, falla descarga/IA/R2 | Par reservado, `procesado=false` | Un webhook nuevo puede reintentar; el sweeper purga a las 24 h |
| Falla el UPSERT | Rollback completo | El par queda reservado y reintentable |
| UPDATE afecta ≠2 filas | Rollback completo | `ErrPairNotReserved` |
| Proceso muere tras subir R2 | Objeto huérfano en R2 | Limpieza R2 pendiente (P1) |
| Dos réplicas ejecutan sweeper | Solo una obtiene el lock | La otra omite la pasada |
| Duplicado del mismo grupo | `duplicate`, sin media | No reprocesa |

**Limitación conocida:** si el par queda reservado y el proceso muere antes del
UPSERT, no se persiste `key`/`message` de la imagen. Un duplicado del mismo
grupo devuelve `duplicate` y no reintenta. Para reintento durable hace falta
guardar el payload de media en `impactos` o usar una cola externa (P1).

---

## 4. Contrato de esquema asumido

El preflight `postgres.VerifySchema` exige estas columnas:

### `impactos`

| Columna | Uso |
|---|---|
| `id` | PK, referenciada por `pareja_id` |
| `hash_largo` | identidad del archivo/mensaje |
| `hash_corto` | presentación |
| `id_grupo` | JID del grupo |
| `caption` | texto crudo del mensaje |
| `instancia` | instancia de Evolution |
| `procesado` | cierre del binomio |
| `recibido_en` | sello de hora Caracas |
| `pareja_id` | reserva mutua del par |

Requisitos adicionales:

```text
UNIQUE (hash_largo, id_grupo)
pareja_id -> impactos(id) ON DELETE SET NULL
índice parcial por (recibido_en) WHERE procesado = TRUE
índice parcial por (recibido_en) WHERE procesado = FALSE
```

### `comprobantes`

| Grupo | Columnas |
|---|---|
| Identidad | `hash_largo` (PK/unique), `hash_corto`, `instancia` |
| Recibo | `monto_recibo`, `moneda_recibo`, `banco`, `titular`, `referencia`, `url_imagen` |
| Binomio | `grupo_1`, `grupo_2`, `caption_1`, `caption_2` |
| Liquidación | `socio_1`, `socio_2`, `tasa_1`, `tasa_2`, `monto_1`, `monto_2`, `me1`, `me2`, `lote_tasa` |
| Tiempo | `fecha_operacion`, `creado_en`, `actualizado_en` |

CHECK esperados:

```text
monto_recibo > 0
tasa_1 > 0 AND tasa_2 > 0
monto_1 <> 0 AND monto_2 <> 0
abs(me1) >= 0.01 AND abs(me2) >= 0.01
grupo_1 <> grupo_2 (normalizado)
socio_1 <> socio_2 (normalizado)
lote_tasa no vacío
```

El código no aplica migraciones: las tablas son propiedad del contrato de
datos. Si el entorno objetivo difiere, `VerifySchema` falla antes de aceptar
tráfico y el SQL está centralizado en
`internal/ingest/postgres/sql.go` y `internal/sweeper/sql.go`.

---

## 5. Pruebas y verificación

Comandos ejecutados:

```text
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
```

Resultado: **todas las pruebas pasan**.

Cobertura funcional relevante:

- `internal/clock`:
  - offset UTC-4 y nombre de zona;
  - fallback `VET` con un loader que falla;
  - `FromUnix` segundos/milisegundos y conversión a Caracas;
  - `WallClock` no desplaza la hora;
  - `ForTimestamp` conserva la hora de pared;
  - medianoche de Caracas.
- `internal/domain`:
  - binomio válido;
  - grupos/socios iguales, placeholders, tasas/importes cero, sin lote, sin
    imagen y sin fecha;
  - normalización de acentos.
- `internal/ingest`:
  - **1x no descarga**: `Descargar`, `Subir`, `Extraer` y `Liquidar` no se
    llaman; se persiste un impacto sin par;
  - duplicado no descarga;
  - segundo grupo descarga exactamente una vez, usa el caption del segundo,
    liquida y guarda el par;
  - fallo de IA no guarda ni marca;
  - par desalineado no descarga;
  - cálculo 2x con dos monedas base y rechazo de socios iguales;
  - parser de Evolution (hash, timestamp, fallback por id, no-imagen, fromMe).
- `internal/ingest/postgres`:
  - SQL de UPSERT y marcado de dos filas;
  - `ErrPairNotReserved` si el UPDATE no afecta dos filas.
- `internal/sweeper`:
  - el SQL no contiene `comprobantes`;
  - solo borra impactos;
  - validación de intervalo y retenciones.
- `internal/config`:
  - defaults de sweeper y rechazo de URLs/claves inválidas.

---

## 6. Operación

| Variable | Default | Uso |
|---|---|---|
| `HOOTY_ADDR` | `:8080` | listen address |
| `DATABASE_URL` | — | PostgreSQL obligatorio |
| `HOOTY_WEBHOOK_TOKEN` | vacío | token opcional del webhook |
| `HOOTY_DEFAULT_INSTANCE` | `EVOLUTION_INSTANCE` | instancia por defecto |
| `HOOTY_MAX_BODY_BYTES` | 25 MiB | límite del webhook |
| `HOOTY_MAX_IMAGE_BYTES` | 20 MiB | límite de imagen |
| `HOOTY_PROCESSING_TIMEOUT` | 2m | descarga + IA + DB |
| `HOOTY_SWEEPER_ENABLED` | true | activa el worker |
| `HOOTY_SWEEPER_INTERVAL` | 1h | ticker |
| `HOOTY_PROCESSED_RETENTION` | 1h | purga procesados |
| `HOOTY_ORPHAN_RETENTION` | 24h | purga huérfanos |
| `EVOLUTION_*` | — | API de Evolution |
| `GEMINI_*` | — | extracción |
| `R2_*` | — | publicación de imagen |

Rutas:

```text
POST /webhooks/evolution
POST /api/v1/webhooks/hooty
GET  /healthz
```

Logs clave:

```text
hooty.ingest.waiting_pair
hooty.ingest.duplicate
hooty.ingest.processed
hooty.sweeper
hooty.sweeper.error
hooty.http.ingest_failed
hooty.shutdown.done
```

---

## 7. Deuda técnica y riesgos

1. **Esquema real:** el repositorio se implementó contra el contrato descrito
   en este ADR. No se ejecutó una inspección de la base de producción desde
   este entorno; `VerifySchema` valida columnas al arrancar. Verificar además
   tipos de fecha y constraints exactos.
2. **Reintento durable:** no se guarda `key`/`message`/payload en `impactos`.
   Un par reservado con fallo no se reprocesa automáticamente. Agregar
   `payload JSONB`/`media_key` o una cola durable.
3. **Precisión monetaria:** `float64` se conserva por compatibilidad con el
   extractor y el motor previo; migrar a `decimal` o enteros de menor unidad.
4. **Liquidación:** polaridad, spread, herencia de perfiles y reglas de abono
   quedan pendientes. La fórmula actual es deliberadamente conservadora.
5. **R2 huérfano:** si la IA o la transacción fallan después del PUT, el objeto
   queda en R2. Agregar limpieza por antigüedad.
6. **Retención de procesados:** 1 h es agresiva para auditoría; evaluar 7–30
   días si hay requisito legal.
7. **Dependencia de perfiles/tasas:** el liquidador asume
   `perfiles_glaukov` y `tasas_glaukov`; si Hooty se despliega con otra base,
   esas tablas deben existir o el adaptador debe sustituirse.
8. **Métricas:** solo hay logs; agregar contadores de pares, fallos de IA,
   latencia y sweeps.
9. **Zona horaria:** el código asume columnas `timestamp without time zone`.
   Si el DBA usó `timestamptz`, revisar `ForTimestamp`/`WallClock` antes del
   canary.

---

## 8. Criterios de aceptación

- [x] Primer impacto: cero descargas, cero llamadas a IA, cero escritura en
      `comprobantes`.
- [x] Segundo impacto de grupo distinto: una descarga, una extracción, una
      liquidación y un UPSERT.
- [x] Dos grupos iguales o un placeholder no pueden formar par.
- [x] El UPSERT y el cierre de los dos impactos comparten transacción.
- [x] El sweeper usa `pg_try_advisory_lock` y no referencia `comprobantes`.
- [x] Fechas y fallback UTC-4 cubiertos por tests.
- [x] `go test ./...`, `go test -race ./...` y `go vet ./...` pasan.
