# Análisis arquitectónico y estado actual de Hooty v2

- **Fecha:** 2026-10-26
- **Repositorio:** `prietocoin/hooty`
- **Rama:** `main`
- **Último commit funcional de referencia:** `b5538d3` (`feat(ui): add PWA install prompt button for mobile devices`)
- **Estado:** monolito modular desplegable; lecturas y PWA implementadas,
  escrituras y endurecimiento pendientes

---

## 1. Resumen ejecutivo

Hooty v2 es un monolito modular escrito en Go que integra en un solo binario:

1. Ingesta transaccional de comprobantes 2x desde Evolution.
2. API de lectura sobre PostgreSQL para el frontend.
3. Frontend estático embebido con `embed.FS`.
4. Sweeper de impactos con advisory lock.
5. PWA instalable en móviles.

El despliegue usa **Nixpacks nativo de Easypanel**, sin `Dockerfile`. El binario
se construye como `./out` y arranca con `./out`. La UI no requiere un servidor
estático separado: se sirve desde el mismo proceso.

La base de datos mantiene un contrato híbrido:

- `comprobantes` es la tabla definitiva de lectura y escritura contable.
- `perfil` y `tasa` son vistas de solo lectura para catálogos.
- Las escrituras de catálogos, cuando se implementen, irán a las tablas físicas
  `perfiles_glaukov` y `tasas_glaukov`, nunca a las vistas.
- `impactos` conserva la sala de espera 2x y la evidencia mínima de WhatsApp.

---

## 2. Arquitectura actual

### 2.1 Monolito modular en Go

```text
cmd/server/              entrada, wiring, mux y graceful shutdown
internal/api/            lectura HTTP, DTO, filtros, CORS y SQL read-only
internal/clock/          hora oficial America/Caracas
internal/config/         variables de entorno y validación
internal/domain/         entidades y reglas contables
internal/ingest/         webhook, parser, lazy download y liquidación
internal/ingest/evolution/ descarga de media
internal/ingest/gemini/  extracción de comprobantes
internal/ingest/postgres/ store transaccional y liquidador SQL
internal/ingest/r2/      publicación SigV4 en R2
internal/sweeper/        retención de impactos
ui/                      frontend estático embebido
ui_assets.go             //go:embed all:ui
nixpacks.toml            build/start de Easypanel
```

El `go.mod` declara Go 1.27.2. El diseño usa patrones de `http.ServeMux` de Go
1.22+ para registrar método y ruta completa.

### 2.2 Enrutamiento HTTP

El mux principal se ensambla en `newMux` y se envuelve con `api.CORS`:

```text
POST /webhooks/evolution
POST /api/v1/webhooks/hooty
GET  /healthz
GET  /api/v1/comprobantes
GET  /api/v1/comprobantes/
GET  /api/v1/comprobantes/{hash}
GET  /api/v1/comprobantes/{hash}/
GET  /api/v1/perfiles
GET  /api/v1/perfiles/
GET  /api/v1/tasas/monitor
GET  /api/v1/tasas/monitor/
GET  /api/v1/reportes/filtros
GET  /api/v1/reportes/filtros/
GET  /{$}       -> 308 /comprobantes/
GET  /          -> FileServer embebido
```

Decisiones relevantes:

- Las rutas de API se registran con método (`GET`, `POST`) para que
  `ServeMux` haga match exacto.
- `OPTIONS` no se registra como patrón. Go 1.22+ puede hacer panic si un
  subárbol `OPTIONS /api/v1/` convive con rutas más específicas. El preflight
  se resuelve en el middleware global `api.CORS`.
- `GET /{$}` es la raíz exacta; `GET /` es el catch-all de archivos estáticos.
  Las rutas más específicas conservan prioridad.
- Los tests de `cmd/server` verifican que `/`, `/comprobantes/` y
  `/api/v1/comprobantes` no colisionen.

### 2.3 Middleware CORS

`internal/api/cors.go`:

- Inyecta `Access-Control-Allow-Origin: *` y cabeceras de lectura.
- Expone `X-Total-Count`, `X-Limit`, `X-Offset` y `X-Has-More`.
- Responde `204 No Content` a cualquier `OPTIONS`.
- Para el resto, delega en el mux.

El CORS es global; cubre API, webhooks y UI. Las escrituras futuras deberán
restringir el origen y los métodos.

---

## 3. Integración del frontend con `embed.FS`

### 3.1 Assets incrustados

`ui_assets.go` declara:

```go
//go:embed all:ui
var uiAssets embed.FS
```

`UIHandler()` usa `fs.Sub(uiAssets, "ui")` para que la raíz del `FileServer`
sea el contenido de `ui/`, no la carpeta `ui/` como prefijo. Así:

```text
GET /comprobantes/  -> ui/comprobantes/index.html
GET /config.js      -> ui/config.js
GET /shared/...     -> ui/shared/...
```

Sin `fs.Sub`, el FileServer esperaría `/ui/comprobantes/`; ese error fue
corregido y cubierto por tests.

### 3.2 Reglas del servidor estático

- `GET /{$}` redirige con `308 Permanent Redirect` a `/comprobantes/`.
- Los HTML y directorios se sirven con `Cache-Control: no-cache`.
- Los assets llevan `?v=` y usan caché de una hora.
- Solo se permiten `GET` y `HEAD`; otros métodos devuelven `405`.
- El handler estático se registra al final para no interceptar healthz,
  webhooks ni API.

### 3.3 Configuración de API en la UI

`ui/config.js` resuelve:

- En `localhost`/`127.0.0.1`/`::1`: `http://localhost:8080`.
- En producción: cadena vacía, para usar el mismo origen servido por Hooty.

Como la UI se sirve desde el mismo binario, no hay CORS en el camino normal;
las cabeceras quedan para desarrollo o integraciones externas.

---

## 4. Modelo de despliegue en Easypanel

### 4.1 Nixpacks nativo, sin Dockerfile

El `Dockerfile` fue eliminado. Easypanel usa Nixpacks, que construye y arranca
el proyecto Go.

Problema resuelto: la presencia de `ui_assets.go` en la raíz hace que Nixpacks
detecte archivos Go raíz y ejecute `go build -o out` sobre el paquete raíz
(`package hooty`, no `main`). Eso genera un archivo no ejecutable y el arranque
falla con:

```text
/bin/bash: line 1: ./out: Permission denied
```

La solución es `nixpacks.toml`:

```toml
[phases.build]
cmds = ["go build -o out ./cmd/server"]

[start]
cmd = "./out"
```

Con esto:

- El build apunta explícitamente a `cmd/server`.
- `./out` es un binario ejecutable real.
- `ui/` está en el contexto de build y `//go:embed` compila los assets.
- `out` está en `.gitignore`; no se versiona ningún binario.

### 4.2 Build y ejecución

```text
Build:  go build -o out ./cmd/server
Start:  ./out
Puerto: HOOTY_ADDR (default :8080)
Health: GET /healthz
```

El proceso requiere las variables de entorno descritas en `.env.example`:
PostgreSQL, Evolution, Gemini, R2 y parámetros del sweeper. Si faltan las
vistas o columnas del contrato, `VerifySchema`/`VerifyViews` fallan al inicio.

### 4.3 PWA y caché

- `GET /comprobantes/manifest.json` y `GET /comprobantes/sw.js` se sirven desde
  el `embed.FS` con MIME correcto.
- `sw.js` no intercepta la API contable.
- El frontend se actualiza con `?v=` y `CACHE_VERSION`.
- La instalación PWA requiere HTTPS; Easypanel ya termina TLS en el dominio.

---

## 5. Estado de la base de datos

### 5.1 Modelo contable

`comprobantes` contiene las columnas definitivas del recibo y de la
liquidación 2x:

```text
hash_largo, hash_corto, instancia,
monto_recibo, moneda_recibo, banco, titular, referencia, url_imagen,
grupo_1, grupo_2, caption_1, caption_2,
socio_1, socio_2, tasa_1, tasa_2, monto_1, monto_2, me1, me2,
lote_tasa, fecha_operacion, creado_en, actualizado_en
```

La lectura no hace JOINs con `comprobantes_raw`, `comprobantes_liq`,
`impactos_raw` ni `registros_raw`.

### 5.2 Contrato híbrido

| Fuente | Uso | Estado |
|---|---|---|
| `comprobantes` | listado, detalle, filtros y reportes | definitivo, lectura/escritura |
| `perfil` | socios, rol, moneda base, monedas y herencia | vista de lectura |
| `tasa` | último lote y mapa de tasas | vista de lectura |
| `perfiles_glaukov` | escritura de perfiles | tabla base, pendiente de API |
| `tasas_glaukov` | escritura de lotes | tabla base, pendiente de API |
| `impactos` | sala de espera 2x y evidencia mínima | definitivo |

`pg_try_advisory_lock` y `pg_advisory_xact_lock` protegen sweeper y pares. Las
fechas se persisten como `timestamp without time zone` en hora de pared de
`America/Caracas`; la API serializa `fecha_operacion` sin `Z`.

### 5.3 API de lectura

`internal/api` implementa:

- Listado paginado con `limit`/`offset`.
- Filtros por socio, rol, rango de `fecha_operacion` y hash.
- `X-Total-Count`, `X-Limit`, `X-Offset`, `X-Has-More`.
- DTO canónico más aliases legacy para la UI.
- `tipo_op1`/`tipo_op2` calculados comparando `moneda_recibo` con la moneda
  base del socio y, si difiere, con la regla `D/P/A` de `perfil.monedas`.
- Búsqueda exacta por hash.
- Perfiles y tasa desde sus vistas.
- Filtros de reportes desde `perfil` y `comprobantes`.

### 5.4 Ingesta y sweeper

- El primer impacto guarda texto/hash sin descargar imagen.
- El segundo impacto de un grupo distinto dispara descarga, R2, IA,
  liquidación y UPSERT atómico.
- El par se reserva con `pareja_id`.
- El sweeper purga procesados >1 h y huérfanos >24 h, sin tocar
  `comprobantes`.

### 5.5 Verificación de esquema

- `postgres.VerifySchema` valida columnas de `impactos` y `comprobantes`.
- `api.VerifyViews` valida `perfil` y `tasa`.
- Si el esquema difiere, el proceso falla al arrancar en vez de servir datos
  incompletos.
- No hay tests de integración contra PostgreSQL real en CI; los tests de SQL
  usan pgxmock y los de rutas usan stores falsos.

---

## 6. Estado actual del frontend

### 6.1 Lectura

La UI consume el DTO híbrido y muestra:

- Listado paginado de comprobantes.
- Filtros por socio, rol, fechas y hash.
- Reportes con paginación real y sin corte de 100.
- Detalle, imagen, tasas y montos congelados.
- Perfiles y monitor de tasas desde las vistas.

### 6.2 PWA

Implementado en `ui/comprobantes/js/pwa.js`:

- Listener `beforeinstallprompt` con `preventDefault`.
- Banner flotante “Instalar App” solo si el evento fue capturado.
- `prompt()` y escucha `userChoice`.
- `appinstalled` y modo `standalone` ocultan la invitación.
- Cierre recordado en `sessionStorage`.
- Manifest con `id`, `start_url`, `scope`, `display: standalone`, orientación e
  íconos 192/512.
- Metas `apple-mobile-web-app-*` y `apple-touch-icon` para Safari iOS.
- Service Worker registrado con `?v=` y servido desde el binario.

Safari iOS no soporta `beforeinstallprompt`; la instalación se hace con
“Añadir a pantalla de inicio”.

### 6.3 Límites

- La UI está en modo lectura: edición/borrado de comprobantes y guardado de
  perfiles/tasas siguen apuntando a endpoints no implementados en Hooty.
- El PWA no tiene estrategia offline de datos contables; el Service Worker
  deliberadamente no cachea la API.

---

## 7. Estado de pruebas y calidad

Ejecutado en local:

```text
go test ./...          OK
go test -race ./...    OK
go vet ./...           OK
go build ./...         OK
node --check pwa.js    OK
```

Cobertura relevante:

- `internal/api`: DTO, `tipo_op`, fechas Caracas, filtros, paginación,
  CORS, rutas exactas y errores sin fuga de SQL.
- `cmd/server`: precedencia del mux, UI embebida, API JSON, webhooks y health.
- `ui_assets`: manifest, SW, PWA y assets.
- `internal/clock`: UTC-4, fallback, `FromUnix`, `WallClock`, `ForTimestamp`.
- `internal/ingest`: 1x sin descarga, 2x, duplicados, fallos y liquidación base.
- `internal/sweeper`: SQL que no toca `comprobantes`.

No hay pruebas E2E contra el dominio de producción ni contra PostgreSQL real.

---

## 8. Riesgos y deuda técnica

1. **Escrituras:** no existen endpoints para editar/borrar comprobantes ni
   guardar perfiles/tasas. La UI debe considerarse de solo lectura.
2. **Contrato de vistas:** `perfil` y `tasa` se validan por columnas al
   arrancar; si el DBA cambia el contrato, el proceso no inicia.
3. **Reintento durable:** no se persiste `key`/`message` de media en
   `impactos`; un fallo después de reservar el par no se reintenta solo.
4. **Precisión monetaria:** se usa `float64`; evaluar decimal o enteros de
   menor unidad.
5. **Liquidación:** polaridad, spread y herencia completas siguen pendientes.
6. **R2 huérfano:** una falla después del PUT deja el objeto sin limpieza.
7. **Observabilidad:** solo logs; faltan métricas de pares, latencia, fallos
   de IA y sweeps.
8. **Seguridad:** las lecturas usan CORS `*`; las escrituras futuras deben
   exigir autenticación y origen restringido.
9. **Despliegue:** verificar que Easypanel reconstruya con `nixpacks.toml` y
   que el proceso ejecute el commit esperado. Un contenedor viejo puede seguir
   respondiendo `/healthz` y devolver 404 en rutas nuevas.
10. **Dockerfile eliminado:** el `.dockerignore` quedó como archivo residual;
    puede retirarse si se confirma que no se usará Docker.

---

## 9. Siguientes pasos recomendados

### P0 — Operación

1. Verificar en Easypanel que el build use `nixpacks.toml` y que `out` sea
   ejecutable.
2. Confirmar que el commit desplegado sirve `/`, `/comprobantes/`,
   `/comprobantes/manifest.json` y `/api/v1/comprobantes`.
3. Revisar logs de arranque de `VerifySchema`/`VerifyViews`.
4. Probar instalación PWA en Chrome Android y “Añadir a pantalla de inicio” en
   Safari iOS.

### P1 — Escrituras y contrato

1. Implementar `PUT/DELETE /api/v1/comprobantes/{hash}` con autenticación.
2. Implementar `POST/PATCH/DELETE /api/v1/perfiles` sobre
   `perfiles_glaukov`.
3. Implementar escritura de lotes sobre `tasas_glaukov`.
4. Definir si `tipo_op1/2` se deriva siempre o se persiste en
   `comprobantes`.
5. Añadir validación de origen, API key o sesión para mutaciones.

### P2 — Robustez

1. Persistir payload de media o usar cola durable para reintentos.
2. Migrar importes a decimal/enteros de menor unidad.
3. Completar reglas de polaridad, spread y herencia.
4. Limpieza de objetos R2 huérfanos.
5. Métricas Prometheus/OpenTelemetry y trazas por hash.
6. Tests de integración con PostgreSQL y contrato de vistas.
7. CI que ejecute tests/build y smoke test post-deploy.

### P3 — PWA

1. Estrategia de actualización del Service Worker y aviso de nueva versión.
2. Página/estado offline mínimo (sin datos contables).
3. Analítica de instalación y eventos `appinstalled`.
4. Revisar iconos maskable y splash screens en iOS/Android.

---

## 10. Conclusión

Hooty v2 ya es un monolito modular desplegable que integra ingesta, lectura,
UI y PWA en un solo binario. La separación entre `internal/api` (lectura),
`internal/ingest` (escritura contable) y `internal/sweeper` (retención) es
clara, y el frontend consume un DTO estable sin joins a tablas raw.

El foco inmediato no es añadir más superficie de lectura, sino **cerrar el ciclo
de escritura**, **verificar el despliegue real** y **endurecer la
observabilidad y los reintentos**. Las decisiones arquitectónicas están
registradas en:

- `docs/adr/002-hooty-ingestion-v2.md`
- `docs/adr/003-ui-and-api-migration.md`
