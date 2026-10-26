# ADR 003 — Migración de la UI al contrato de Hooty v2

- **Estado:** aceptado e implementado en esta fase (lecturas)
- **Fecha:** 2026-10-25
- **Alcance:** UI copiada en `hooty/ui`, API de lectura de Hooty v2,
  compatibilidad de campos y retiro progresivo de aliases legacy
- **Relacionado:** `docs/adr/002-hooty-ingestion-v2.md`

---

## 1. Contexto y copia de la UI

Se copió `C:\Proyectos\glauk-ui` a `C:\Proyectos\hooty\ui` con autorización
explícita.

- 132 archivos copiados.
- Excluidos: `.git`, `node_modules`, `.next`, `dist`.
- El proyecto original se verificó por SHA-256: los 132 archivos quedaron
  idénticos.
- No se modificó el proyecto original.

La UI es un frontend estático. **No contiene consultas SQL directas.** Toda la
lectura se hace por HTTP contra endpoints del backend. Por eso la migración de
tablas se divide en:

1. API de lectura en Hooty que consulta `comprobantes`, `perfil` y `tasa`.
2. DTO de compatibilidad para no romper la UI actual.
3. Ajustes de la UI para consumir campos canónicos.

---

## 2. Diagnóstico de referencias legacy

### 2.1 Referencias literales a tablas viejas

| Archivo | Línea | Referencia | Cambio |
|---|---:|---|---|
| `ui/tasas/index.html` | 64 | `Guardar en tasas_glaukov` | `Guardar en tasa` |
| `ui/tasas/js/views/monitor.js` | 39 | `Guardando en tasas_glaukov...` | `Guardando en tasa...` |
| `ui/comprobantes/js/modules/comprobantes/services/saldosCalculatorService.js` | 3 | Comentario `comprobantes_final` | `comprobantes` |

No hay otras apariciones literales de `comprobantes_raw`, `comprobantes_liq`,
`perfiles_glaukov` o `tasas_glaukov` en `ui/`.

### 2.2 Endpoints consumidos por la UI

```text
GET    /api/v1/comprobantes
GET    /api/v1/comprobantes/{hash}        (usado de forma indirecta por hash)
PUT    /api/v1/comprobantes/{hash}
POST   /api/v1/comprobantes/{hash}/releer
DELETE /api/v1/comprobantes/{hash}

GET    /api/v1/perfiles
POST   /api/v1/perfiles/guardar
PATCH  /api/v1/perfiles/{nombre}/estado
DELETE /api/v1/perfiles/{nombre}

GET    /api/v1/tasas/monitor
GET    /api/v1/tasas/socios
POST   /api/v1/tasas/guardar-lote
POST   /api/v1/tasas/publicar-lote
POST   /api/v1/tasas/disparar
GET    /api/tasas/ultimas

GET    /api/v1/reportes/filtros
POST   /api/v1/reportes/imagen
POST   /api/v1/reportes/texto

GET    /api/directorio
POST   /api/socios/config
PATCH  /api/socios/{nombre}/estado
PATCH  /api/socios/desactivar-todos
POST   /api/socios/guardar-vigentes
POST   /api/socios/restaurar-vigentes
DELETE /api/directorio/{nombre}
```

En esta fase se implementan las **lecturas**. Las escrituras de comprobantes y
catálogos quedan documentadas como pendientes para una fase posterior.

---

## 3. Decisiones aprobadas

### D1 — Hooty v2 es el backend único de lectura

Se aprueba la Opción A: Hooty expone las rutas GET de lectura y la UI apunta a
Hooty o a un gateway del mismo origen.

Rutas implementadas:

```text
GET /api/v1/comprobantes
GET /api/v1/comprobantes/{hash}
GET /api/v1/perfiles
GET /api/v1/tasas/monitor
GET /api/v1/reportes/filtros
```

### D2 — DTO de compatibilidad

Las respuestas incluyen campos canónicos y aliases legacy para que la UI actual
siga funcionando mientras se migra componente por componente.

Ejemplo de fila:

```json
{
  "hash_largo": "...",
  "hash_corto": "...",
  "monto_recibo": 100,
  "monto": 100,
  "monto_local": 100,
  "moneda_recibo": "USD",
  "moneda": "USD",
  "moneda_local": "USD",
  "fecha_operacion": "2026-10-25T10:00:00",
  "fecha_hora_comprobante": "2026-10-25T10:00:00",
  "socio_1": "CHASAN",
  "nombre_socio_1": "CHASAN",
  "socio_2": "NELSY",
  "nombre_socio_2": "NELSY",
  "grupo_1": "...",
  "grupo_2": "...",
  "tasa_1": 40,
  "tasa_2": 41,
  "monto_1": 4000,
  "monto_2": 4100,
  "me1": 100,
  "me2": 100,
  "m1_socio": 100,
  "m2_socio": 100,
  "lote_tasa": "T070",
  "lote_tasa_asignado": "T070",
  "lote": "T070",
  "url_imagen": "https://...",
  "url_r2_comprobante": "https://...",
  "link_img": "https://...",
  "tipo_op1": "D",
  "tipo_op2": "A",
  "es_x2": true
}
```

Los aliases se retirarán cuando todos los componentes consuman los campos
canónicos.

### D3 — Escrituras de catálogos

Las escrituras **no usarán las vistas**. Cuando se implementen, apuntarán a las
tablas físicas subyacentes:

```text
perfil  -> perfiles_glaukov
tasa    -> tasas_glaukov
```

Las vistas `perfil` y `tasa` son la fuente de lectura; las tablas base son la
fuente de escritura.

### D4 — Naturaleza y tipo de operación

`comprobantes` no almacena `tipo_op1`/`tipo_op2`. El API los calcula en el DTO:

1. Si `moneda_recibo` coincide con `moneda_base` del socio: `A`.
2. Si no coincide, se consulta `perfil.monedas[moneda_recibo].tipo` (o `def`).
3. Si no hay regla o no es `D`/`P`/`A`: `D`.

Esto preserva la visualización D/P/A de la UI sin modificar el esquema de
`comprobantes`.

### D5 — Paginación real

Se elimina el tope rígido de 500 en el listado y el corte de 100 en reportes.

- `GET /api/v1/comprobantes` acepta `limit` y `offset`.
- `limit` por defecto: 20.
- `limit` máximo: 200.
- Se devuelven cabeceras:

```text
X-Total-Count
X-Limit
X-Offset
X-Has-More
```

La UI debe paginar hasta consumir el total, sin truncar en 500.

### D6 — Fecha de negocio en Caracas

Todas las consultas y filtros usan `comprobantes.fecha_operacion`, que se
interpreta como hora de pared de `America/Caracas`.

- El backend recibe `fechaInicio`/`fechaFin` como `YYYY-MM-DD`.
- El filtro es inclusivo por día:

```sql
c.fecha_operacion >= $inicio::date
AND c.fecha_operacion < ($fin::date + INTERVAL '1 day')
```

- El DTO serializa `fecha_operacion` sin `Z` ni offset, en formato
  `YYYY-MM-DDTHH:mm:ss`.
- El cliente no debe convertir esa fecha a UTC.

---

## 4. Contrato de lectura

### 4.1 `GET /api/v1/comprobantes`

Parámetros:

| Parámetro | Tipo | Uso |
|---|---|---|
| `socio` | string | coincide con `socio_1` o `socio_2` |
| `rol` | string | filtra socios por `perfil.rol` |
| `fechaInicio` | `YYYY-MM-DD` | inicio inclusivo de `fecha_operacion` |
| `fechaFin` | `YYYY-MM-DD` | fin inclusivo de `fecha_operacion` |
| `hash` | string | prefijo de `hash_largo` o `hash_corto` |
| `orden` | string | `fecha_asc`, `fecha_desc`, `monto_asc`, `monto_desc` |
| `limit` | int | 1..200, default 20 |
| `offset` | int | default 0 |

Respuesta: array raíz de comprobantes.

### 4.2 `GET /api/v1/comprobantes/{hash}`

- Búsqueda exacta por `hash_largo`.
- `404` si no existe.
- Mismo DTO de compatibilidad.

### 4.3 `GET /api/v1/perfiles`

Lee desde la vista `perfil`:

```text
nombre, id_grupo, rol, moneda_base, saldo_inicial, mostrar, monedas, herencia
```

Devuelve además aliases usados por la UI:

```text
roles, moneda_socio, whatsapp, saldo_anterior
```

### 4.4 `GET /api/v1/tasas/monitor`

Lee el último registro de la vista `tasa`:

```json
{
  "id_tasa": "T070",
  "tasas": { "USD": 1, "VES": 40, "COP": 4000 }
}
```

La columna de orden se detecta entre `id`, `creado_en`, `created_at`, `fecha` e
`id_tasa`; si no existe una columna temporal, se usa `id_tasa`.

### 4.5 `GET /api/v1/reportes/filtros`

```json
{
  "roles": ["ADMIN", "SOCIO"],
  "entidades": ["CHASAN", "NELSY"],
  "hashes": [
    { "value": "8a3e...", "label": "8A3EF797" }
  ]
}
```

- `roles` y `entidades` salen de `perfil`.
- `hashes` salen de `comprobantes`.

---

## 5. SQL de referencia

### 5.1 Listado sin JOINs a raw/liq

```sql
SELECT
    c.hash_largo,
    c.hash_corto,
    c.instancia,
    c.monto_recibo,
    c.moneda_recibo,
    c.banco,
    c.titular,
    c.referencia,
    c.url_imagen,
    c.grupo_1,
    c.grupo_2,
    c.caption_1,
    c.caption_2,
    c.socio_1,
    c.socio_2,
    c.tasa_1,
    c.tasa_2,
    c.monto_1,
    c.monto_2,
    c.me1,
    c.me2,
    c.lote_tasa,
    c.fecha_operacion,
    c.creado_en,
    c.actualizado_en
FROM comprobantes c
WHERE ...
ORDER BY c.fecha_operacion DESC, c.hash_largo DESC
LIMIT $n OFFSET $m;
```

### 5.2 Perfiles

```sql
SELECT
    nombre,
    COALESCE(id_grupo, '') AS id_grupo,
    COALESCE(rol, 'SOCIO') AS rol,
    COALESCE(moneda_base, 'USDT') AS moneda_base,
    COALESCE(saldo_inicial, 0) AS saldo_inicial,
    COALESCE(mostrar, '{}'::jsonb) AS mostrar,
    COALESCE(monedas, '{}'::jsonb) AS monedas,
    COALESCE(herencia, FALSE) AS herencia
FROM perfil
ORDER BY nombre;
```

### 5.3 Tasa vigente

```sql
SELECT id_tasa, COALESCE(tasas, '{}'::jsonb)
FROM tasa
ORDER BY <columna-detectada> DESC
LIMIT 1;
```

### 5.4 Filtros de reportes

```sql
SELECT DISTINCT COALESCE(rol, '')
FROM perfil
WHERE COALESCE(rol, '') <> ''
ORDER BY 1;

SELECT nombre
FROM perfil
WHERE COALESCE(nombre, '') <> ''
ORDER BY nombre;

SELECT hash_largo, hash_corto
FROM comprobantes
ORDER BY fecha_operacion DESC, hash_largo DESC;
```

---

## 6. Archivos de la UI a modificar

### 6.1 Lectura de comprobantes

| Archivo | Cambio |
|---|---|
| `ui/config.js` | Apuntar a Hooty en local y a mismo origen en producción |
| `ui/comprobantes/js/modules/comprobantes/services/comprobantesMapperService.js` | Priorizar `monto_recibo`, `moneda_recibo`, `fecha_operacion`, `url_imagen`, `lote_tasa`, `socio_1/2`; mantener aliases de salida |
| `ui/comprobantes/js/modules/comprobantes/services/comprobantesApiService.js` | Orden por `monto_recibo`; fallback de fecha `fecha_operacion`; PUT con `lote_tasa` |
| `ui/comprobantes/js/modules/comprobantes/services/comprobantesCargaService.js` | Lote activo dinámico; no asumir `T070` |
| `ui/comprobantes/js/modules/comprobantes/services/comprobantesFilterService.js` | `monto_recibo`; `fecha_operacion` |
| `ui/comprobantes/js/modules/comprobantes/states/comprobantesState.js` | `url_imagen`; `monto_recibo`; eliminar tope 500 |
| `ui/comprobantes/js/modules/comprobantes/states/comprobantesReporteState.js` | `moneda_recibo`; `lote_tasa`; `fecha_operacion` |
| `ui/comprobantes/js/modules/comprobantes/utils/comprobantesFechaUtils.js` | `fecha_operacion` primero |
| `ui/comprobantes/js/modules/comprobantes/utils/comprobantesFormatters.js` | Fallback `monto_recibo` |
| `ui/comprobantes/js/modules/comprobantes/utils/comprobantesTasasUtils.js` | `moneda_recibo`; `tipo_op1/2` provisto por API |
| `ui/comprobantes/js/modules/comprobantes/services/comprobantesNaturalezaService.js` | `moneda_recibo`; `caption_1/2` |
| `ui/comprobantes/js/modules/comprobantes/utils/saldosFilterUtils.js` | `monto_recibo`; `fecha_operacion` |
| `ui/comprobantes/js/modules/comprobantes/services/saldosCalculatorService.js` | Comentario y fallback `monto_recibo` |
| `ui/comprobantes/index.html` | Payload/detalle con campos canónicos; conservar aliases de lectura |

### 6.2 Perfiles y directorio

| Archivo | Cambio |
|---|---|
| `ui/perfiles/js/api.js` | Backend sirve `/api/v1/perfiles` desde `perfil` |
| `ui/comprobantes/js/modules/directorio/services/directorioService.js` | Migrar `/api/directorio` a `/api/v1/perfiles` |
| `ui/comprobantes/js/modules/directorio/services/socioConfigModalService.js` | Migrar `/api/socios/config` a `/api/v1/perfiles/guardar` (fase de escritura) |
| `ui/comprobantes/js/modules/directorio/services/socioHerenciaService.js` | Igual |
| `ui/comprobantes/js/modules/directorio/services/socioEstadoService.js` | Migrar a `/api/v1/perfiles/{nombre}/estado` (fase de escritura) |
| `ui/comprobantes/js/modules/directorio/services/directorioAccionesService.js` | Migrar acciones masivas a `/api/v1/perfiles/*` |
| `ui/perfiles/js/domain.js`, `components/modal-socio.js`, `components/socio-card.js` | Sin cambio si `perfil` conserva columnas |

### 6.3 Tasas

| Archivo | Cambio |
|---|---|
| `ui/tasas/js/api.js` | Backend lee `tasa`; escrituras a `tasas_glaukov` en fase posterior |
| `ui/tasas/index.html` | Texto `tasa` |
| `ui/tasas/js/views/monitor.js` | Texto `tasa`; conservar `id_tasa`/`tasas` |
| `ui/tasas/js/views/socios.js` | Campos desde `perfil` |
| `ui/comprobantes/js/modules/tasas/services/tasasMercadoService.js` | `/api/v1/tasas/monitor` sobre `tasa` |
| `ui/perfiles/js/api.js` | `/api/tasas/ultimas` sobre `tasa` |

### 6.4 Reportes

| Archivo | Cambio |
|---|---|
| `ui/reportes/js/api.js` | `/reportes/filtros` y `/comprobantes` sobre las tablas nuevas |
| `ui/reportes/js/views/constructor.js` | `fecha_operacion`, `socio_1`, `monto_recibo`, `moneda_recibo`, `lote_tasa` |
| `ui/reportes/js/components/filtros.js` | Consume roles/entidades/hashes del API nuevo |
| `ui/reportes/js/main.js` | `api.socios()` sobre `perfil` |
| `ui/reportes/js/views/constructor.js` | Paginación real, no `limit: 100, offset: 0` fijo |

### 6.5 Caché y límites

| Archivo | Cambio |
|---|---|
| `ui/comprobantes/js/modules/comprobantes/states/comprobantesState.js` | Eliminar `capMaximoDOM: 500` |
| `ui/comprobantes/js/modules/comprobantes/services/comprobantesApiService.js` | Revisar `MAXIMO_CONSULTA`/`MAXIMO_RECUPERACION` |
| `ui/comprobantes/sw.js`, `ui/perfiles/sw.js`, `ui/reportes/sw.js`, `ui/tasas/sw.js` | Subir versión de caché |
| `ui/comprobantes/js/pwa.js` | Subir `SW_VERSION` |

---

## 7. Implementación en Hooty

Se crea el paquete `internal/api/`:

```text
internal/api/dto.go          DTO canónico + aliases, tipo_op, fechas Caracas
internal/api/repository.go   SQL de lectura y detección de orden de tasa
internal/api/handler.go      rutas GET, filtros, paginación, CORS
internal/api/dto_test.go
internal/api/handler_test.go
```

Wiring en `cmd/server/main.go`:

```go
readStore, err := api.NewRepository(ctx, pool)
readAPI := api.NewHandler(readStore, logger)
readAPI.Register(mux)
```

`Register` usa patrones de método y ruta completa de Go 1.22+:

```go
mux.HandleFunc("GET /api/v1/comprobantes", h.ListComprobantes)
mux.HandleFunc("GET /api/v1/comprobantes/{$}", h.ListComprobantes)
mux.HandleFunc("GET /api/v1/comprobantes/{hash}", h.GetComprobante)
mux.HandleFunc("GET /api/v1/comprobantes/{hash}/{$}", h.GetComprobante)
mux.HandleFunc("GET /api/v1/perfiles", h.ListPerfiles)
mux.HandleFunc("GET /api/v1/tasas/monitor", h.LatestTasa)
mux.HandleFunc("GET /api/v1/reportes/filtros", h.ReportFilters)
```

Los preflight `OPTIONS` se resuelven en el middleware global `api.CORS(mux)`,
no con un patrón `OPTIONS /api/v1/`. Go 1.22+ rechaza ese patrón cuando
convive con rutas más específicas del mismo subárbol, como
`/api/v1/webhooks/hooty`.

Cabeceras de paginación y CORS:

```text
X-Total-Count
X-Limit
X-Offset
X-Has-More
Access-Control-Expose-Headers: X-Total-Count, X-Limit, X-Offset, X-Has-More
```

---

## 8. Riesgos y pendientes

1. **Escrituras:** `PUT/DELETE /comprobantes`, perfiles y tasas siguen
   pendientes. La UI debe considerarse de solo lectura en esta fase.
2. **Vistas:** confirmar que `perfil` y `tasa` expongan las columnas asumidas.
   Si no, ajustar la vista o el SELECT antes de desplegar.
3. **`tipo_op`:** el cálculo depende de `perfil.monedas`; si el perfil no
   existe o no tiene la moneda, el DTO devuelve `D`.
4. **Zona horaria:** el DTO y los filtros asumen `timestamp without time zone`
   en hora de Caracas. Si la base usa `timestamptz`, revisar la serialización.
5. **CORS:** Hooty expone las lecturas con `Access-Control-Allow-Origin: *`;
   las escrituras futuras deben restringirse.
6. **Caché PWA:** subir versiones para evitar que el navegador conserve el
   contrato anterior.
7. **Límite 517:** la UI debe paginar hasta `X-Total-Count`, sin tope local de
   500.

---

## 9. Criterios de aceptación

- [x] `GET /api/v1/comprobantes` responde array y cabeceras de paginación.
- [x] Filtros por socio, rol, rango de `fecha_operacion` y hash funcionan.
- [x] `GET /api/v1/comprobantes/{hash}` devuelve 200/404 correctamente.
- [x] `GET /api/v1/perfiles` lee de `perfil`.
- [x] `GET /api/v1/tasas/monitor` lee el último registro de `tasa`.
- [x] `GET /api/v1/reportes/filtros` devuelve roles, entidades y hashes.
- [x] El DTO incluye campos canónicos y aliases legacy.
- [x] `tipo_op1/2` se calcula con `moneda_recibo` y `perfil`.
- [x] Las fechas se serializan en hora de Caracas sin `Z`.
- [x] `go test ./...`, `go test -race ./...` y `go vet ./...` pasan.
- [x] `ui/config.js` apunta a Hooty en local y a mismo origen en producción.
- [x] El mapper prioriza campos canónicos.
- [ ] Escrituras de comprobantes, perfiles y tasas (fase posterior).

## 10. Estado de implementación

Backend:

```text
internal/api/dto.go
internal/api/repository.go
internal/api/handler.go
internal/api/dto_test.go
internal/api/handler_test.go
```

Rutas registradas en `cmd/server/main.go`:

```text
GET /api/v1/comprobantes
GET /api/v1/comprobantes/{hash}
GET /api/v1/perfiles
GET /api/v1/tasas/monitor
GET /api/v1/reportes/filtros
```

UI:

- `ui/config.js` resuelve Hooty local en `:8080` y mismo origen en producción.
- El mapper prioriza campos canónicos y conserva aliases de salida.
- Se eliminó el tope local de 500.
- Reportes pagina por `LIMIT/OFFSET` hasta `X-Total-Count`.
- El directorio y el monitor de tasas usan las rutas nuevas.

Verificación ejecutada:

```text
go test ./...        OK
go test -race ./...  OK
go vet ./...         OK
node --check ...     OK
```

Limitación vigente: las escrituras (edición/borrado de comprobantes, guardado de
perfiles y tasas) siguen sin endpoint en Hooty. La UI debe operar en modo
lectura hasta implementar esa fase.

## 11. Diagnóstico del 404 en `hooty.jairokov.com`

El cuerpo observado era `404 page not found` con
`Content-Type: text/plain`. Ese texto lo genera el `http.ServeMux` cuando no
encuentra un patrón, no el handler de la API. El handler devuelve
`{"error":"not_found"}` con `Content-Type: application/json`.

Causas posibles, en orden:

1. La instancia desplegada servía un commit anterior a la incorporación de
   `internal/api` (el commit `a4b6ee8` no tenía rutas de lectura).
2. El mux registraba solo el subtree `"/api/v1/"`; se reemplazó por patrones
   exactos con método de Go 1.22+.
3. Un proxy que elimina `/api/v1` antes de llegar al contenedor. En ese caso
   el backend recibe `/comprobantes` y devuelve 404.

Se corrigió el registro a patrones completos:

```text
GET /api/v1/comprobantes
GET /api/v1/comprobantes/
GET /api/v1/comprobantes/{hash}
GET /api/v1/comprobantes/{hash}/
GET /api/v1/perfiles
GET /api/v1/tasas/monitor
GET /api/v1/reportes/filtros
OPTIONS (middleware global api.CORS, sin patrón en ServeMux)
```

Validación local:

```text
TestRegisterRutasExactasYBarraFinal        OK
TestRegisterNoPaniegaConWebhookYCORSGlobal OK
TestRegisterRechazaMetodoNoPermitido       OK
```

Si tras el redespliegue el 404 continúa, revisar:

- que el contenedor ejecute el commit `58bec6e` o posterior;
- los logs de arranque (`VerifyViews`, `VerifySchema`);
- la configuración de strip-prefix/rewrite del proxy;
- que `/healthz` y `/api/v1/comprobantes` lleguen al mismo upstream.

## 12. UI como monolito modular embebido

La UI se sirve desde el propio binario Go:

- `ui_assets.go` declara `//go:embed all:ui`.
- `UIHandler()` usa `fs.Sub` para quitar el prefijo `ui/` y `http.FileServer`
  para servir los assets.
- `GET /` se registra como `GET /{$}` y redirige con 308 a `/comprobantes/`.
- El handler estático se monta al final en `mux.Handle("GET /", uiHandler)`, por
  lo que las rutas más específicas (`/healthz`, webhooks y `/api/v1/*`)
  conservan prioridad.
- `fs.Sub(embeddedUI, "ui")` quita el prefijo `ui/`: el FileServer ve
  `/comprobantes/`, no `/ui/comprobantes/`.
- `Dockerfile` usa un build multi-stage, copia todo el contexto (incluido
  `ui/`), compila `./cmd/server` a `/out/hooty`, copia a `/app/out`, aplica
  `chmod 0755` y arranca con `CMD ["/app/out"]`.
- `nixpacks.toml` fija el build de Easypanel en `go build -o out ./cmd/server`
  y el start en `./out`. Sin esto, Nixpacks detecta el paquete Go de la raíz
  (`ui_assets.go`) y genera un archivo no ejecutable, causando
  `./out: Permission denied`.
- El runtime del Dockerfile usa Debian slim con shell y CA roots; así el start
  command de Easypanel (`./out`) funciona desde `/app`.
- `out` está en `.gitignore` y `.dockerignore`; no se versiona ningún binario.

Tests:

```text
TestUIHandlerSirveLaRaizYLosAssets              OK
TestMuxEstaticoNoColisionaConAPIYWebhooks       OK
TestMuxSirveUIYNoRompeAPI                       OK
TestMuxConservaWebhookYHealth                   OK
go test ./...                                   OK
go test -race ./...                             OK
go vet ./...                                    OK
go build ./...                                  OK
```
