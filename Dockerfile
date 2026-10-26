# Build stage: incluye ui/ para que //go:embed all:ui compile los assets.
FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/hooty \
    ./cmd/server

# Runtime mínimo: binario estático + CA roots para Evolution/Gemini/R2.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/hooty /hooty

EXPOSE 8080

ENTRYPOINT ["/hooty"]
