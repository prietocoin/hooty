// Package ingest implementa el webhook de Evolution y la ingesta transaccional
// 2x. La regla central es lazy download: el primer impacto de un hash nunca
// toca la imagen; solo el segundo impacto de un grupo distinto dispara la
// descarga, la IA, la liquidación y la escritura atómica del comprobante.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"hooty/internal/clock"
	"hooty/internal/domain"
)

// ErrMediaUnavailable indica que el binario ya no está disponible en Evolution.
var ErrMediaUnavailable = errors.New("ingest: media no disponible")

// Options configura el servicio.
type Options struct {
	Logger            *slog.Logger
	ProcessingTimeout time.Duration
	MaxImageBytes     int64
}

// Service coordina la sala de espera y el procesamiento 2x.
type Service struct {
	store     Store
	media     MediaFetcher
	images    ImageStore
	extractor Extractor
	liq       Liquidator
	clock     clock.Clock
	log       *slog.Logger
	timeout   time.Duration
	maxImage  int64
}

// NewService valida las dependencias y construye el servicio.
func NewService(store Store, media MediaFetcher, images ImageStore,
	extractor Extractor, liq Liquidator, clk clock.Clock, opts Options) (*Service, error) {
	if store == nil {
		return nil, errors.New("ingest: Store es obligatorio")
	}
	if media == nil {
		return nil, errors.New("ingest: MediaFetcher es obligatorio")
	}
	if images == nil {
		return nil, errors.New("ingest: ImageStore es obligatorio")
	}
	if extractor == nil {
		return nil, errors.New("ingest: Extractor es obligatorio")
	}
	if liq == nil {
		return nil, errors.New("ingest: Liquidator es obligatorio")
	}
	if clk == nil {
		return nil, errors.New("ingest: Clock es obligatorio")
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.ProcessingTimeout <= 0 {
		opts.ProcessingTimeout = 2 * time.Minute
	}
	return &Service{
		store:     store,
		media:     media,
		images:    images,
		extractor: extractor,
		liq:       liq,
		clock:     clk,
		log:       opts.Logger,
		timeout:   opts.ProcessingTimeout,
		maxImage:  opts.MaxImageBytes,
	}, nil
}

// Recibir registra el impacto y, si completa un binomio de grupos distintos,
// ejecuta el pipeline pesado. El primer impacto retorna sin descargar bytes.
func (s *Service) Recibir(ctx context.Context, event Evento) (domain.Decision, error) {
	event = event.Normalize()
	if err := event.Validate(); err != nil {
		return domain.Decision{}, err
	}

	now := s.clock.Now()
	if now.IsZero() {
		return domain.Decision{}, errors.New("ingest: el reloj devolvió una hora cero")
	}
	impacto := domain.NormalizeImpacto(domain.Impacto{
		HashLargo:  event.HashLargo,
		HashCorto:  event.HashCorto,
		IDGrupo:    event.IDGrupo,
		Caption:    event.Caption,
		Instancia:  event.Instancia,
		Procesado:  false,
		RecibidoEn: now,
	})
	if err := impacto.Validate(); err != nil {
		return domain.Decision{}, err
	}

	registro, err := s.store.RegistrarImpacto(ctx, impacto)
	if err != nil {
		return domain.Decision{}, fmt.Errorf("ingest: registrar impacto: %w", err)
	}

	decision := domain.Decision{
		Status: registro.Status,
		Hash:   impacto.HashLargo,
		Conteo: registro.Conteo,
	}
	if decision.Conteo <= 0 {
		decision.Conteo = 1
	}

	switch registro.Status {
	case domain.StatusWaitingPair:
		s.log.Info("hooty.ingest.waiting_pair",
			"hash", impacto.HashLargo,
			"grupo", impacto.IDGrupo,
			"conteo", decision.Conteo,
		)
		return decision, nil
	case domain.StatusDuplicate:
		s.log.Info("hooty.ingest.duplicate",
			"hash", impacto.HashLargo,
			"grupo", impacto.IDGrupo,
		)
		return decision, nil
	case domain.StatusPairReady:
		if len(registro.Pair) != 2 {
			return decision, fmt.Errorf("ingest: par_ready con %d impactos", len(registro.Pair))
		}
		if err := s.process(ctx, registro.Pair, event); err != nil {
			return decision, err
		}
		decision.Status = domain.StatusProcessed
		decision.Conteo = 2
		return decision, nil
	default:
		return decision, fmt.Errorf("ingest: estado desconocido %q", registro.Status)
	}
}

func (s *Service) process(ctx context.Context, pair []domain.Impacto, event Evento) error {
	first, second := pair[0], pair[1]
	caption2 := firstNonEmpty(second.Caption, event.Caption)
	if domain.Canonical(first.IDGrupo) == domain.Canonical(second.IDGrupo) {
		return domain.Validationf("el par debe tener dos grupos distintos")
	}
	if domain.Canonical(second.IDGrupo) != domain.Canonical(event.IDGrupo) {
		return fmt.Errorf("ingest: el segundo impacto no corresponde al evento actual")
	}
	if first.ID <= 0 || second.ID <= 0 {
		return errors.New("ingest: el par no tiene identificadores persistidos")
	}
	if len(event.Key) == 0 || len(event.Message) == 0 {
		return errors.New("ingest: faltan key/message para descargar la imagen")
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	started := time.Now()
	image, mimeType, err := s.media.Descargar(ctx, event.Instancia, event.Key, event.Message)
	if err != nil {
		return fmt.Errorf("ingest: descargar imagen del segundo impacto: %w", err)
	}
	if len(image) == 0 {
		return fmt.Errorf("ingest: la imagen del segundo impacto llegó vacía: %w", ErrMediaUnavailable)
	}
	if s.maxImage > 0 && int64(len(image)) > s.maxImage {
		return fmt.Errorf("ingest: imagen de %d bytes excede el límite de %d", len(image), s.maxImage)
	}
	mimeType = firstNonEmpty(mimeType, event.MimeType, "image/jpeg")

	objectKey := "comprobantes/" + event.HashLargo + extensionFor(mimeType)
	url, err := s.images.Subir(ctx, objectKey, image, mimeType)
	if err != nil {
		return fmt.Errorf("ingest: subir imagen: %w", err)
	}

	extraccion, err := s.extractor.Extraer(ctx, image, mimeType, caption2)
	if err != nil {
		return fmt.Errorf("ingest: extraer comprobante: %w", err)
	}
	extraccion = extraccion.Normalize()
	if err := extraccion.Validate(); err != nil {
		return err
	}

	now := s.clock.Now()
	comprobante, err := s.liq.Liquidar(ctx, domain.LiquidacionInput{
		HashLargo:      event.HashLargo,
		HashCorto:      event.HashCorto,
		Instancia:      event.Instancia,
		Extraccion:     extraccion,
		URLImagen:      url,
		Grupo1:         first.IDGrupo,
		Grupo2:         second.IDGrupo,
		Caption1:       first.Caption,
		Caption2:       caption2,
		FechaOperacion: first.RecibidoEn,
		RecibidoEn:     now,
	})
	if err != nil {
		return fmt.Errorf("ingest: liquidar: %w", err)
	}

	// La identidad del binomio y los datos del recibo son responsabilidad de
	// este servicio, no del liquidador. Así un adaptador no puede desviar el
	// snapshot hacia otro hash o grupo.
	comprobante = domain.NormalizeComprobante(comprobante)
	comprobante.HashLargo = event.HashLargo
	comprobante.HashCorto = event.HashCorto
	comprobante.Instancia = event.Instancia
	comprobante.MontoRecibo = extraccion.Monto
	comprobante.MonedaRecibo = extraccion.Moneda
	comprobante.Banco = extraccion.Banco
	comprobante.Titular = extraccion.Titular
	comprobante.Referencia = extraccion.Referencia
	comprobante.URLImagen = url
	comprobante.Grupo1 = first.IDGrupo
	comprobante.Grupo2 = second.IDGrupo
	comprobante.Caption1 = first.Caption
	comprobante.Caption2 = caption2
	comprobante.FechaOperacion = first.RecibidoEn
	comprobante.CreadoEn = now
	comprobante.ActualizadoEn = now
	comprobante = domain.NormalizeComprobante(comprobante)
	if math.IsNaN(comprobante.MontoRecibo) || comprobante.MontoRecibo <= 0 {
		return domain.Validationf("el comprobante no tiene monto positivo")
	}
	if err := comprobante.Validate(); err != nil {
		return err
	}

	ids := []int64{first.ID, second.ID}
	if err := s.store.GuardarComprobanteYMarcar(ctx, comprobante, ids); err != nil {
		return fmt.Errorf("ingest: guardar comprobante y marcar par: %w", err)
	}

	s.log.Info("hooty.ingest.processed",
		"hash", comprobante.HashLargo,
		"grupo_1", comprobante.Grupo1,
		"grupo_2", comprobante.Grupo2,
		"lote_tasa", comprobante.LoteTasa,
		"duracion_ms", time.Since(started).Milliseconds(),
	)
	return nil
}

func extensionFor(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
