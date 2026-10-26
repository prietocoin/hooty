# Inspección exhaustiva del pipeline de Hooty v2

## 1. Metadatos de la inspección

| Campo | Valor |
|---|---|
| Fecha | 2026-10-26 |
| Repositorio | `prietocoin/hooty` |
| Rama | `main` |
| Commit base (`HEAD`) | `01ee32b` (local; incluye DATE-01, SWEEP-01 y DB-01) |
| Documentos de referencia | `docs/adr/002-hooty-ingestion-v2.md`, `docs/adr/003-ui-and-api-migration.md`, `docs/analisis-hooty.md` |
| Alcance | Ingesta, Gemini, liquidación 2x, PostgreSQL, catálogos, sweeper, API HTTP, UI embebida, PWA y despliegue Nixpacks |
| Método | Lectura de código, SQL, tests y contratos; ejecución local de `go test ./...`, `go test -race ./...`, `go build ./...` y `go vet ./...` |
| Cambios aplicados | DATE-01, SWEEP-01, DB-01 y SCHEMA-01 corregidos; matriz recalculada. Commit local sin push |

### 1.1 Resultado global

La inspección cubre **17 reglas** del pipeline, puntuadas así:

- `[CUMPLE]` = 1.0
- `[PARCIAL]` = 0.5
- `[NO CUMPLE]` = 0.0

Resultado:

| Veredicto | Cantidad |
|---|---:|
| CUMPLE | 15 |
| PARCIAL | 2 |
| NO CUMPLE | 0 |
| **Total** | **17** |

**Cumplimiento global: 94.1%**  
Cálculo: `(15 × 1.0 + 2 × 0.5 + 0 × 0.0) / 17 = 16 / 17 = 0.9412`.

| Fase | Puntaje | Porcentaje |
|---|---:|---:|
| 1. Ingesta y webhooks | 2.5 / 3 | 83.3% |
| 2. Extracción con IA | 2.0 / 2 | 100.0% |
| 3. Conciliación y liquidación | 2.5 / 3 | 83.3% |
| 4. Base de datos y catálogos | 3.0 / 3 | 100.0% |
| 5. Sweeper | 2.0 / 2 | 100.0% |
| 6. HTTP, UI y PWA | 4.0 / 4 | 100.0% |

### 1.2 Resumen ejecutivo

El flujo 2x, el lazy download, el DTO híbrido, la UI embebida y el despliegue
Nixpacks están bien resueltos. El pipeline es funcional para el camino feliz:
el primer impacto no descarga media, el segundo de otro grupo dispara el
procesamiento, la escritura final es atómica, las fechas se manejan en Caracas
y el sweeper no toca `comprobantes`.

Hallazgos resueltos en esta actualización:

1. **DATE-01 [CUMPLE]:** `FechaOperacion` ahora usa `second.RecibidoEn`, el
   impacto que cierra el par. Tests en `service_test.go` lo verifican.
2. **SWEEP-01 [CUMPLE]:** `sqlDeleteOrphans` exige `pareja_id IS NULL`; los
   pares fallidos solo se purgan cuando ambos lados superaron la retención.
3. **DB-01 [CUMPLE]:** el liquidador lee `perfil` y `tasa`, no las tablas
   físicas `perfiles_glaukov`/`tasas_glaukov`.
4. **SCHEMA-01 [CUMPLE]:** `VerifySchema` y `VerifyViews` validan `data_type`
   compatible y exigen `timestamp without time zone` en las fechas contables.

Brechas que permanecen:

1. **P1 — Reintentos:** no hay payload durable de media; si Gemini/R2 falla
   después de reservar el par, un duplicado no lo reprocesa y el par espera al
   sweeper.
2. **P1 — Seguridad futura:** CORS es `*` y no hay autenticación en las rutas
   actuales; es aceptable porque son lecturas, pero bloquea las mutaciones de
   Hooty v3.
3. **P1 — Atomicidad completa:** la reserva y el procesamiento no comparten
   transacción; la escritura final sí es atómica.

Deuda separada de SCHEMA-01: validar constraints, índices y FKs específicas;
la presente corrección cubre columnas y familias de `data_type`.

### 1.3 Evidencia de ejecución local

```text
go test ./...        OK
go test -race ./...  OK
go build ./...       OK
go vet ./...         OK
```

Los tests no incluyen PostgreSQL real ni E2E de producción. Los tests de SQL
usan `pgxmock`; los de rutas usan stores falsos.

---

## 2. Cuestionario de auditoría por fases

### Fase 1 — Ingesta y webhooks (Evolution API)

| ID | Pregunta / Regla | Política esperada | Veredicto | Evidencia | Riesgo |
|---|---|---|---|---|---|
| F1-01 | ¿Aplica lazy download? ¿El impacto 1x guarda solo texto/hash sin descargar media de Evolution? | El primer impacto no debe llamar a Evolution, R2 ni Gemini. | **[CUMPLE]** | `internal/ingest/service.go:85-149` devuelve `waiting_pair`/`duplicate` antes de `process`; `Service.process` solo se invoca en `StatusPairReady` (línea 136). `TestPrimerImpactoNoDescargaNiProcesaImagen` en `internal/ingest/service_test.go` verifica 0 descargas y 0 bytes. | Bajo. El primer impacto persiste hash, texto, grupo, instancia y `recibido_en`, sin media. |
| F1-02 | ¿R2 y Gemini se disparan estrictamente en el impacto 2x de un grupo distinto? | Solo el segundo impacto, con par real y grupos distintos, debe descargar, subir y analizar. | **[CUMPLE]** | `Store.RegistrarImpacto` inserta con `ON CONFLICT (hash_largo,id_grupo)` y busca contraparte con `id_grupo <> $2`, `procesado = FALSE`, `pareja_id IS NULL` (`internal/ingest/postgres/sql.go:15-38`). `Service.process` verifica grupos distintos y que el segundo coincida con el evento actual (`service.go:151-158`). `TestSegundoGrupoDescargaUnaVezYLiquida` cubre una descarga única. | Bajo en el camino feliz. Si el par se reserva y falla el procesamiento, la media ya se descargó (ver F1-03). |
| F1-03 | ¿Cómo se manejan duplicados, reintentos de webhooks y colisiones de pares? | Duplicados sin reprocesar; pares sin colisión; reintentos durables. | **[PARCIAL]** | `service.go:130-135` devuelve `duplicate` sin media. `Store.RegistrarImpacto` usa `pg_advisory_xact_lock(hashtextextended(...))`, `ON CONFLICT`, `pareja_id` mutuo y `FOR UPDATE SKIP LOCKED` (`store.go:48-129`; `sql.go:10,29-46`). Pero no se persiste `key`/`message`/payload en `impactos`; un duplicado de un par fallido no reprocesa. | Alto: un fallo de Gemini/R2/DB deja el par reservado sin reintento durable; el segundo mensaje puede quedar inutilizado hasta la purga de 24 h. El lock evita colisiones, pero no la pérdida de trabajo. |

### Fase 2 — Extracción con IA (Gemini)

| ID | Pregunta / Regla | Política esperada | Veredicto | Evidencia | Riesgo |
|---|---|---|---|---|---|
| F2-01 | ¿El prompt y el schema garantizan captura determinista de monto, moneda, banco, referencia y fecha? | La fecha de operación se define por el impacto que cierra el par; los demás campos deben ser estables. | **[CUMPLE]** | `internal/ingest/service.go:209,233-235` usa `second.RecibidoEn` para `LiquidacionInput.FechaOperacion` y para el `Comprobante`. `internal/ingest/service_test.go:260,270` valida ambos. Gemini sigue extrayendo monto, moneda, banco y referencia; no extrae fecha impresa. | Bajo para la política acordada: la fecha contable es el segundo impacto en Caracas. La fecha impresa en el recibo queda como deuda separada, no como incumplimiento de DATE-01. |
| F2-02 | ¿Cómo reacciona ante timeouts, cuotas agotadas o fallos transitorios de red? | Timeout, reintentos y rotación de claves/modelos ante 429/5xx/red. | **[CUMPLE]** | `internal/ingest/gemini/client.go:48-61` configura timeout (45 s por defecto) y cliente HTTP. `Extraer` recorre modelos y hasta 3 intentos por modelo, rota claves y aplica backoff (`client.go:95-117`). `retryable` cubre 429, 500, 502, 503 y 504 (`client.go:165-172`); errores de red y de respuesta se reintentan. `Service.process` añade timeout global de 2 min por defecto (`service.go:166`). | Medio: no hay circuit breaker, métricas de cuota ni reintento durable tras agotar la petición. La reacción inmediata es correcta; la recuperación posterior no. |

### Fase 3 — Conciliación contable y liquidación 2x

| ID | Pregunta / Regla | Política esperada | Veredicto | Evidencia | Riesgo |
|---|---|---|---|---|---|
| F3-01 | ¿La reserva del par, la liquidación y la inserción contable ocurren en una transacción atómica? | Una sola unidad transaccional para reservar, liquidar e insertar. | **[PARCIAL]** | `Store.RegistrarImpacto` reserva el par y hace commit (`store.go:48-129`). Después, `Service.process` descarga, sube a R2, llama a Gemini y liquida fuera de PostgreSQL (`service.go:171-241`). Finalmente `Store.GuardarComprobanteYMarcar` abre otra transacción, hace UPSERT y exige exactamente 2 filas (`store.go:148-189`). | Alto: la escritura contable + cierre del par es atómica, pero la reserva no comparte transacción con el procesamiento. Un fallo intermedio deja el par reservado y sin comprobante; no hay reintento durable. R2 puede quedar con un objeto huérfano. |
| F3-02 | ¿Se garantiza la política horaria de `America/Caracas` sin sufijos `Z` ni desvíos UTC? | Hora de pared de Caracas en columnas contables; serialización sin `Z`; filtros por `fecha_operacion`. | **[CUMPLE]** | `internal/clock/clock.go` implementa `Caracas()`, `ForTimestamp`, `WallClock` y fallback UTC-4 con `time/tzdata`. El pool configura `timezone=America/Caracas` (`cmd/server/main.go:openPool`). Las fechas se escriben con `clock.ForTimestamp` (`store.go`), se leen con `WallClock` y el DTO usa `fechaLocal` sin `Z` (`internal/api/dto.go:fechaLocal`). El filtro de lectura usa `fecha_operacion` (`internal/api/repository.go:buildWhere`). | Bajo: la zona es correcta y `fecha_operacion` usa el segundo impacto (DATE-01). La fecha impresa en el comprobante sigue como deuda separada. |
| F3-03 | ¿Se usan advisory locks para evitar concurrencia entre instancias? | Lock por hash en ingesta y lock exclusivo en sweeper. | **[CUMPLE]** | Ingesta: `sqlAdvisoryLock = SELECT pg_advisory_xact_lock(hashtextextended($1, 0))` (`internal/ingest/postgres/sql.go:10`) ejecutado en `RegistrarImpacto` (`store.go:60`). Sweeper: `pg_try_advisory_lock($1)` con clave `0x484F4F54595357` (`internal/sweeper/sweeper.go:25,173`). | Bajo. Los locks protegen por hash y por worker; la purga de pares quedó corregida en F5-02. |

### Fase 4 — Base de datos y catálogos

| ID | Pregunta / Regla | Política esperada | Veredicto | Evidencia | Riesgo |
|---|---|---|---|---|---|
| F4-01 | ¿`comprobantes` es la única fuente de verdad contable, sin JOINs con `_raw` o `_liq`? | Listado, detalle y reportes deben leer solo `comprobantes`; sin `comprobantes_raw`/`comprobantes_liq`. | **[CUMPLE]** | `internal/api/repository.go` construye `SELECT ... FROM comprobantes c` (`sqlComprobanteColumns`, `ListComprobantes`, `GetComprobante`). No hay referencias a `comprobantes_raw`/`comprobantes_liq` en el código Go de Hooty; la ingesta escribe directo en `comprobantes` (`internal/ingest/postgres/sql.go:50-88`). | Bajo. El contrato de lectura está alineado con ADR-003. |
| F4-02 | ¿Las consultas de socios y tasas leen estrictamente de las vistas `perfil` y `tasa`? | Toda lectura de catálogos debe usar las vistas; las tablas físicas quedan para escritura. | **[CUMPLE]** | La API usa `FROM perfil` (`repository.go:175-183`) y `FROM tasa` (`repository.go:209-223`). DB-01 migró el liquidador: `sqlProfileByGroup` usa `FROM perfil` y `sqlLatestRateLot` usa `FROM tasa` (`internal/ingest/postgres/sql.go:100-110`); `NewLiquidator` detecta la columna de orden de `tasa` y `TestCatalogQueriesUseCanonicalViews` verifica que no haya referencias a `perfiles_glaukov`/`tasas_glaukov`. | Bajo. La lectura de catálogos queda alineada con ADR-003; las tablas físicas solo se usarán para escritura. |
| F4-03 | ¿El arranque aborta si columnas o tipos no coinciden con el contrato? | Validación de columnas y `data_type` compatible; timestamps críticos sin zona. | **[CUMPLE]** | `postgres.VerifySchema` y `api.VerifyViews` se ejecutan antes de aceptar tráfico (`cmd/server/main.go:54-62`). Ambos leen `information_schema.columns.data_type` y delegan en `dbschema.Validate` (`store.go:193-240`; `repository.go:51-110`). `recibido_en`, `fecha_operacion`, `creado_en` y `actualizado_en` exigen `timestamp without time zone`; las familias texto/entero/booleano/numérico/JSON se validan con compatibilidad. Los tests `TestRequiredSchemaProtectsCaracasTimestamps`, `TestRequiredSchemaRejectsWrongTypeAndMissingColumn`, `TestRequiredViewSchemaValidaTipos` y `internal/dbschema` cubren tipos incorrectos y columnas faltantes. | Bajo para tipos. El error incluye tabla.columna, tipo esperado y tipo encontrado. Constraints/índices/FKs quedan como deuda P2 separada. |

### Fase 5 — Sweeper (limpieza y retención)

| ID | Pregunta / Regla | Política esperada | Veredicto | Evidencia | Riesgo |
|---|---|---|---|---|---|
| F5-01 | ¿El worker usa advisory lock exclusivo para no colisionar entre ejecuciones? | Solo una instancia ejecuta la pasada; lock liberado en la misma conexión. | **[CUMPLE]** | `internal/sweeper/sweeper.go:173-185` adquiere `pg_try_advisory_lock($1)` sobre una conexión dedicada (`pool.Acquire`), ejecuta la transacción y libera con `pg_advisory_unlock` en la misma conexión. `TestSQLDelSweeperNoTocaComprobantes` y `TestNewValidaLaConfiguracion` cubren parte del contrato. | Bajo. La contención se resuelve omitiendo la pasada; no hay colisión de workers. |
| F5-02 | ¿Respeta la purga (procesados >1 h, huérfanos >24 h) y el aislamiento absoluto sobre `comprobantes`? | Borrar solo impactos; procesados >1 h; huérfanos >24 h; no tocar comprobantes; no borrar pares con un lado reciente. | **[CUMPLE]** | `sqlDeleteOrphans` exige `pareja_id IS NULL`; `sqlDeleteFailedPairs` exige que ambos lados estén `procesado = FALSE` y `recibido_en < $1`. `RunOnce` ejecuta ambas consultas en la misma transacción y reporta `Pares` (`internal/sweeper/sql.go:10-38`; `sweeper.go:207-223`). `TestDeleteOrphansSQLProtegeParesViejosYNuevos` y el test gated `TestSweeperNoBorraParViejoNuevo` cubren el par viejo+nuevo. | Bajo. Un par viejo+nuevo queda intacto; solo se purga cuando ambos lados superaron la retención. El test de integración requiere `HOOTY_TEST_DATABASE_URL`. |

### Fase 6 — Servidor HTTP, frontend embebido y PWA

| ID | Pregunta / Regla | Política esperada | Veredicto | Evidencia | Riesgo |
|---|---|---|---|---|---|
| F6-01 | ¿El enrutamiento en `http.ServeMux` (Go 1.22+) previene panics de colisión entre UI y API? | Patrones de método y ruta exactos; OPTIONS fuera del mux; sin conflictos de subárbol. | **[CUMPLE]** | `cmd/server/main.go:newMux` registra `POST /webhooks/evolution`, `POST /api/v1/webhooks/hooty`, las rutas GET del API, `GET /healthz`, `GET /{$}` y `GET /` (estático). `internal/api/cors.go` resuelve `OPTIONS` en middleware, no con un patrón. `TestRegisterNoPaniegaConWebhookYCORSGlobal`, `TestRegisterRutasExactasYBarraFinal` y `TestMuxEstaticoNoColisionaConAPIYWebhooks` cubren precedencia y preflight. | Bajo. La separación de responsabilidades evita el panic de Go 1.22+. |
| F6-02 | ¿El DTO híbrido calcula `tipo_op1`/`tipo_op2` y polaridades en memoria sin alterar el esquema? | Compatibilidad de UI sin agregar columnas a `comprobantes`. | **[CUMPLE]** | `internal/api/dto.go` define `TipoOp1`, `TipoOp2`, `Polaridad1`, `Polaridad2`; `NewComprobanteDTO` los calcula con `tipoOperacion` y `polaridad` (`dto.go:172-229,314-368`). No hay migración ni columna nueva en `comprobantes`. | Bajo en el contrato. La regla `tipoOperacion` es una aproximación D/P/A: no replica la excepción de IRIS ni el caption explícito de la UI anterior; puede clasificar mal algún caso. |
| F6-03 | ¿La raíz `GET /{$}` redirige con 308 y expone `manifest.json` y `sw.js`? | 308 a `/comprobantes/`; assets PWA servidos desde el binario. | **[CUMPLE]** | `cmd/server/main.go:205` registra `GET /{$}` con `http.StatusPermanentRedirect` a `/comprobantes/`; `ui_assets.go` usa `//go:embed all:ui` y `fs.Sub`. `TestMuxSirveUIYNoRompeAPI` verifica 308, HTML y API; `TestManifestYServiceWorkerEmbebidos` verifica manifest y SW; `TestUIHandlerSirveLaRaizYLosAssets` verifica 200 de manifest, SW, icono y `pwa.js`. | Bajo. La UI y la PWA se sirven del mismo binario. |
| F6-04 | ¿El build de despliegue depende exclusivamente de `nixpacks.toml` apuntando a `./cmd/server`? | Nixpacks nativo, sin Dockerfile; binario `./out` ejecutable. | **[CUMPLE]** | `nixpacks.toml` declara `go build -o out ./cmd/server` y `./out` como start. No existe `Dockerfile`. `/out` está en `.gitignore`; no hay binarios versionados (`git ls-files out*` vacío). `ui_assets_test` y `cmd/server/main_test` validan la UI embebida. | Bajo en el repositorio. Depende de que Easypanel use `nixpacks.toml` y de que el runtime Nixpacks tenga los CA roots necesarios. `.dockerignore` quedó residual, sin efecto. |

---

## 3. Matriz de hallazgos y plan de acción

### 3.1 Clasificación de riesgos

| ID | Criticidad | Hallazgo | Evidencia | Impacto | Acción recomendada |
|---|---|---|---|---|---|
| SWEEP-01 | **P0 Crítico (resuelto)** | El sweeper podía borrar un par recién formado cuando uno de sus lados superaba las 24 h. | `internal/sweeper/sql.go:17-38`; `sweeper.go:207-223` | Pérdida de un par válido y fallo de `GuardarComprobanteYMarcar`; trabajo de R2/IA descartado. | **Implementado:** `sqlDeleteOrphans` exige `pareja_id IS NULL` y los pares fallidos solo se purgan si ambos lados son antiguos. |
| DB-01 | **P1 Alto (resuelto)** | El liquidador leía tablas físicas en lugar de las vistas `perfil`/`tasa`. | `internal/ingest/postgres/sql.go:100-110`; `liquidator.go` (nueva detección de orden); contraste con `internal/api/repository.go:175-223` | Divergencia de catálogos, herencia o normalización entre lectura y liquidación. | **Implementado:** `sqlProfileByGroup` usa `FROM perfil` y `sqlLatestRateLot` usa `FROM tasa`; test de contrato SQL. |
| DATE-01 | **P1 Alto (resuelto)** | `fecha_operacion` se tomaba de `first.RecibidoEn`. | `internal/ingest/service.go:209,233-235`; `service_test.go:260,270` | Reportes en el día equivocado; conciliación y auditoría temporal inexactas. | **Implementado:** la fecha se toma de `second.RecibidoEn`, el impacto que cierra el par. |
| RETRY-01 | **P1 Alto** | No hay reintento durable después de reservar el par. | `domain.Impacto` no tiene payload de media; `store.go:48-129`; `service.go:171-245` | Un fallo de R2/Gemini/DB deja el par reservado y sin comprobante; un duplicado no reprocesa. | Persistir `media_key`/payload o usar cola durable; definir reprocesamiento idempotente y estado de intento. |
| SCHEMA-01 | **P1 Alto (resuelto)** | `VerifySchema`/`VerifyViews` validaban solo nombres de columnas. | `internal/dbschema/schema.go`; `internal/ingest/postgres/store.go:193-240`; `internal/api/repository.go:51-110` | Tipos desalineados podían pasar el arranque y fallar en runtime. | **Implementado:** validación de `data_type` por familia y timestamps contables estrictos sin zona; tests de tipos y columnas faltantes. Constraints/índices quedan P2. |
| SEC-01 | **P1 Alto** | CORS `*` y ausencia de autenticación en rutas actuales. | `internal/api/cors.go`; `cmd/server/main.go` | Lecturas contables expuestas a cualquier origen; las futuras mutaciones serían inseguras. | Restringir orígenes, añadir token/sesión y autorización por rol antes de habilitar escrituras. |
| ATOMIC-01 | **P1 Alto** | La reserva y el procesamiento no comparten una transacción. | `store.go:48-129` (reserva); `service.go:171-245`; `store.go:148-189` (escritura final) | Par reservado sin comprobante ante fallos intermedios. | Aceptar el diseño de dos transacciones con reintento durable, o introducir una máquina de estados/claim explícita. |
| DTO-01 | **P2 Deuda** | `tipo_op` del DTO no replica la excepción IRIS/caption de la UI anterior. | `internal/api/dto.go:tipoOperacion`; `ui/comprobantes/js/modules/comprobantes/services/comprobantesNaturalezaService.js` | Posible clasificación D/P/A incorrecta en casos IRIS o caption explícito. | Unificar la regla de naturaleza en el backend y cubrirla con fixtures. |
| FIN-01 | **P2 Deuda** | Importes en `float64` y liquidación base sin polaridad/spread completos. | `internal/domain/domain.go`; `internal/ingest/liquidacion.go` | Errores de coma flotante y diferencias con el motor histórico. | Migrar a decimal/enteros; completar reglas financieras y tests dorados. |
| OBS-01 | **P2 Deuda** | Solo hay logs; no hay métricas ni trazas. | `internal/*`; `cmd/server/main.go` | Difícil detectar sweeps lentos, fallos de IA o latencia de R2. | Instrumentar Prometheus/OpenTelemetry y alertas por cola/errores/latencia. |
| R2-01 | **P2 Deuda** | Un fallo después del PUT deja un objeto huérfano en R2. | `internal/ingest/service.go` (subida antes de IA/DB); `internal/ingest/r2` | Coste de almacenamiento y objetos sin referencia. | Job de limpieza por antigüedad o subida después de una reserva durable. |
| TEST-01 | **P2 Deuda** | No hay tests de integración con PostgreSQL real ni E2E de despliegue. | `internal/ingest/postgres` (pgxmock); `cmd/server/main_test.go` (stores falsos) | El contrato de vistas, tipos y Nixpacks puede fallar solo en producción. | Añadir testcontainer/DB de staging y smoke test post-deploy. |
| DEPLOY-01 | **P2 Deuda** | `.dockerignore` residual sin `Dockerfile`; no se fija la versión de Go de Nixpacks. | Raíz del repo; `nixpacks.toml`; `go.mod` | Confusión de mantenimiento o diferencias de toolchain. | Eliminar `.dockerignore` o documentarlo; fijar `GOTOOLCHAIN`/versión de Nixpacks si aplica. |

### 3.2 Plan de acción

#### P0 — Corregido

1. **SWEEP-01 implementado:** `sqlDeleteOrphans` exige `pareja_id IS NULL` y
   `sqlDeleteFailedPairs` solo purga pares con ambos lados antiguos.
2. **Regresión:** `TestSweeperNoBorraParViejoNuevo` cubre viejo+nuevo cuando
   existe `HOOTY_TEST_DATABASE_URL`; `TestDeleteOrphansSQLProtegeParesViejosYNuevos`
   valida el contrato SQL sin base.

#### P1 — Estado

1. **DB-01 implementado:** el liquidador lee `perfil`/`tasa` y detecta la
   columna de orden de `tasa`.
2. **DATE-01 implementado:** `FechaOperacion` sale de `second.RecibidoEn`.
   Persistir una fecha impresa del recibo sigue como deuda separada.
3. **Pendiente — reintento durable:** guardar `media_key`/payload en
   `impactos` o usar una cola idempotente; permitir reprocesar un par
   reservado sin un webhook nuevo.
4. **SCHEMA-01 implementado:** `VerifySchema`/`VerifyViews` validan
   `data_type` y timestamps sin zona. Queda como P2 validar constraints, FKs e
   índices específicos.
5. **Pendiente — seguridad de mutaciones:** restringir CORS, añadir
   token/sesión, autorización por rol, rate limiting y auditoría.

#### P2 — Deuda programada

1. Migrar `float64` a decimal/enteros y completar polaridad/spread/herencia.
2. Métricas Prometheus/OpenTelemetry para sweeper, IA, R2 y cola.
3. Limpieza de R2 huérfano.
4. Tests de integración con PostgreSQL y E2E de despliegue.
5. Unificar `tipo_op` con la regla IRIS/caption o eliminar la duplicación.
6. PWA: aviso de actualización, estado offline mínimo y validación en
   dispositivos reales.
7. Limpiar `.dockerignore` residual y documentar la versión de Go de Nixpacks.

### 3.3 Criterios de aceptación para cerrar la inspección

- [x] `sqlDeleteOrphans` no borra pares con un lado reciente; test de regresión.
- [x] Todas las lecturas de catálogos usan `perfil`/`tasa`.
- [x] `fecha_operacion` tiene una fuente de negocio definida y probada: el segundo impacto.
- [ ] Existe reintento durable o se documenta formalmente su ausencia.
- [x] El preflight valida columnas y tipos críticos, incluidos timestamps `without time zone`.
- [ ] Validar constraints, FKs e índices específicos (deuda P2).
- [ ] Las mutaciones tienen autenticación y CORS restringido.
- [ ] Hay métricas y smoke test de despliegue.
- [x] `go test ./...`, `go test -race ./...`, `go vet ./...` y `go build ./...` siguen limpios.

---

## 4. Conclusión

Hooty v2 ha resuelto la parte más visible del problema: un solo binario, UI
embebida, lectura canónica, fechas de Caracas, PWA y despliegue Nixpacks. El
camino feliz de la ingesta 2x está bien protegido por locks, `pareja_id` y una
transacción final que exige dos filas.

La inspección no encontró un fallo generalizado de lectura ni de rutas. Los
cuatro hallazgos estructurales detectados quedaron corregidos: SWEEP-01 (par
viejo+nuevo), DATE-01 (fecha del segundo impacto), DB-01 (catálogos por
vistas) y SCHEMA-01 (tipos y timestamps de pared). El pipeline vuelve a estar
alineado con ADR-002 y ADR-003 en esos puntos.

Las brechas que aún condicionan la madurez son los reintentos durables, la
atomicidad completa reserva/procesamiento, la seguridad de las futuras
mutaciones, los constraints/índices y la observabilidad. Con esos frentes
cerrados, Hooty estará en condiciones de avanzar hacia Hooty v3 con escrituras
seguras y una operación medible.
