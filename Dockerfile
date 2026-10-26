# Build stage: incluye ui/ para que //go:embed all:ui compile los assets.
FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p /out && CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/hooty \
    ./cmd/server

# Runtime con shell y /bin/bash: Easypanel puede ejecutar `./out` desde /app.
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/hooty /app/out
RUN chmod 0755 /app/out

EXPOSE 8080

CMD ["/app/out"]
