# Hooty v2

Microservicio independiente de ingesta para comprobantes 2x. Recibe el webhook
de Evolution, mantiene una sala de espera en PostgreSQL y solo procesa la
imagen cuando llega el segundo mensaje de un grupo distinto.

## Principios

- **Lazy download:** el primer impacto persiste texto, hash e identidad; no se
  descarga ni se sube ningún byte de imagen.
- **Binomio explícito:** dos grupos distintos, dos socios reales y `pareja_id`
  mutuo antes de llamar a la IA.
- **Atomicidad:** el `UPSERT` de `comprobantes` y el marcado de exactamente dos
  impactos ocurren en la misma transacción.
- **Hora oficial:** todas las marcas operativas se generan en
  `America/Caracas` (UTC-4), con fallback `VET` si falta tzdata.
- **Sweeper aislado:** solo borra de `impactos`; nunca toca `comprobantes`.

## Estructura

```text
cmd/server/                  Entrada, wiring y healthcheck
internal/clock/              Hora de Caracas, FromUnix, WallClock
internal/domain/             Impacto, Comprobante, Extracción y validaciones
internal/ingest/             Servicio transaccional, parser, HTTP, liquidación
internal/ingest/evolution/   Descarga de media desde Evolution
internal/ingest/gemini/      Extracción con IA
internal/ingest/postgres/    Store atómico, liquidador SQL y preflight
internal/ingest/r2/          PUT firmado SigV4 a Cloudflare R2
internal/sweeper/            Ticker de 1 h y advisory lock
docs/adr/002-...             Decisiones, flujo y deuda técnica
```

## Configuración

Copiar `.env.example` al gestor de secretos y exportar las variables. No hay
valores por defecto para credenciales.

Mínimo necesario:

```text
DATABASE_URL=postgres://...
EVOLUTION_API_URL=https://...
EVOLUTION_INSTANCE=Glauk
GEMINI_API_KEY=...
R2_ACCOUNT_ID=...            # o R2_ENDPOINT
R2_ACCESS_KEY_ID=...
R2_SECRET_ACCESS_KEY=...
R2_BUCKET_NAME=remesas-img
```

El servicio verifica al arrancar que `impactos` y `comprobantes` tengan las
columnas del contrato. Si falta `pareja_id`, si quedó declarado `NOT NULL` o
si no existe la unicidad `(hash_largo, id_grupo)`, corregir el esquema antes
de desplegar.

## Ejecutar

```powershell
go test ./...
go vet ./...
go run ./cmd/server
```

Endpoints:

- `POST /webhooks/evolution` y `POST /api/v1/webhooks/hooty`: webhook.
- `GET /healthz`: ping a PostgreSQL.

Autenticación opcional:

```text
X-Hooty-Token: <HOOTY_WEBHOOK_TOKEN>
```

Sin `HOOTY_WEBHOOK_TOKEN` el webhook queda abierto; el arranque emite un WARN.

## Respuestas del webhook

```json
{"status":"waiting_pair","hash":"...","conteo":1}
{"status":"duplicate","hash":"...","conteo":1}
{"status":"processed","hash":"...","conteo":2}
{"status":"ignored"}
```

Los mensajes que no son imagen devuelven `200 ignored`. Un payload inválido
devuelve `400`. Una violación de dominio devuelve `422`.

## Tests

```text
go test ./...        # fechas, fallback, 1x sin descarga, 2x, fallos, SQL del sweeper
go test -race ./...
go vet ./...
```

## Documentación

La decisión completa, el flujo transaccional y la deuda técnica están en
[`docs/adr/002-hooty-ingestion-v2.md`](docs/adr/002-hooty-ingestion-v2.md).
