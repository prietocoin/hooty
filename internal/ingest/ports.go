package ingest

import (
	"context"
	"encoding/json"

	"hooty/internal/domain"
)

// Store es la sala de espera transaccional. RegistrarImpacto debe serializar
// por hash y reservar el par con pareja_id; GuardarComprobanteYMarcar debe
// escribir el snapshot y marcar exactamente dos impactos en la misma
// transacción.
type Store interface {
	RegistrarImpacto(ctx context.Context, impacto domain.Impacto) (domain.Registro, error)
	GuardarComprobanteYMarcar(ctx context.Context, comprobante domain.Comprobante, ids []int64) error
}

// MediaFetcher descarga el binario desde Evolution. Solo se invoca para el
// segundo impacto de un binomio listo.
type MediaFetcher interface {
	Descargar(ctx context.Context, instancia string, key, message json.RawMessage) ([]byte, string, error)
}

// ImageStore publica la imagen y devuelve la URL que quedará en comprobantes.
// La clave debe ser determinista para que un reintento sobrescriba el objeto.
type ImageStore interface {
	Subir(ctx context.Context, key string, data []byte, mimeType string) (string, error)
}

// Extractor convierte la imagen en los cinco campos del comprobante.
type Extractor interface {
	Extraer(ctx context.Context, image []byte, mimeType, caption string) (domain.Extraccion, error)
}

// Liquidator resuelve socios y tasas y congela el snapshot 2x.
type Liquidator interface {
	Liquidar(ctx context.Context, input domain.LiquidacionInput) (domain.Comprobante, error)
}
