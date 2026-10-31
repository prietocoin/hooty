// Package publisher despacha las carteleras de tasas a los socios activos.
package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

// Socio es un destino de publicación con su chat_id de WhatsApp.
type Socio struct {
	Nombre     string
	ChatID     string
	MonedaBase string
	Monedas    json.RawMessage
}

// Lote es la instantánea de tasas que se publica.
type Lote struct {
	ID    string
	Tasas map[string]float64
}

// Cartelera es el contenido que se renderiza para un destinatario.
type Cartelera struct {
	Socio      string
	ChatID     string
	MonedaBase string
	Monedas    json.RawMessage
	Lote       string
	Tasas      map[string]float64
	Prueba     bool
	Generada   time.Time
}

// Envio es una cartelera ya rasterizada por el cliente.
type Envio struct {
	Socio  string
	ChatID string
	PNG    []byte
}

// Source obtiene el lote vigente y los socios activos (ON).
type Source interface {
	LatestLote(ctx context.Context) (Lote, error)
	ActiveSocios(ctx context.Context) ([]Socio, error)
}

// SocioFinder permite resolver un socio puntual aunque no esté en la lista ON.
type SocioFinder interface {
	SocioByName(ctx context.Context, nombre string) (Socio, error)
}

// Sender entrega una imagen a un chat_id concreto.
type Sender interface {
	Send(ctx context.Context, chatID string, png []byte, caption string) error
}

// Renderer convierte una cartelera en PNG.
type Renderer interface {
	Render(Cartelera) ([]byte, error)
}

// Detalle es el resultado por destinatario.
type Detalle struct {
	Socio  string `json:"socio"`
	ChatID string `json:"chat_id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Result es la respuesta HTTP de publicación.
type Result struct {
	IDTasa     string    `json:"id_tasa"`
	Procesados int       `json:"procesados"`
	Fallidos   int       `json:"fallidos"`
	Prueba     bool      `json:"prueba"`
	Detalle    []Detalle `json:"detalle"`
}

// Publisher orquesta lectura, renderizado y despacho.
type Publisher struct {
	source     Source
	sender     Sender
	renderer   Renderer
	testChatID string
	log        *slog.Logger
	now        func() time.Time
}

// New construye el publicador. testChatID es el grupo de pruebas obligatorio
// cuando is_prueba viene encendido.
func New(source Source, sender Sender, renderer Renderer, testChatID string, logger *slog.Logger) *Publisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Publisher{
		source:     source,
		sender:     sender,
		renderer:   renderer,
		testChatID: strings.TrimSpace(testChatID),
		log:        logger,
		now:        time.Now,
	}
}

// Publish despacha según el modo. En prueba solo existe un destino: el grupo de
// pruebas. En producción se envía una cartelera a cada socio activo.
func (p *Publisher) Publish(ctx context.Context, isPrueba bool) (Result, error) {
	lote, err := p.source.LatestLote(ctx)
	if err != nil {
		return Result{}, err
	}
	resultado := Result{IDTasa: lote.ID, Prueba: isPrueba}

	if isPrueba {
		if p.testChatID == "" {
			return resultado, errors.New("publisher: falta configurar EVOLUTION_TEST_GROUP para publicar en prueba")
		}
		socios, err := p.source.ActiveSocios(ctx)
		if err != nil {
			return resultado, err
		}
		if len(socios) == 0 {
			return resultado, errors.New("publisher: no hay socios activos ON para publicar")
		}
		for _, socio := range socios {
			destino := socio
			destino.ChatID = p.testChatID
			detalle := p.enviar(ctx, destino, lote, true)
			resultado.Detalle = append(resultado.Detalle, detalle)
			if detalle.Status == "ok" {
				resultado.Procesados++
			} else {
				resultado.Fallidos++
			}
		}
		sort.Slice(resultado.Detalle, func(i, j int) bool {
			return resultado.Detalle[i].Socio < resultado.Detalle[j].Socio
		})
		if resultado.Procesados == 0 {
			return resultado, errors.New("publisher: fallaron todos los envíos de prueba")
		}
		return resultado, nil
	}

	socios, err := p.source.ActiveSocios(ctx)
	if err != nil {
		return resultado, err
	}
	if len(socios) == 0 {
		return resultado, errors.New("publisher: no hay socios activos ON para publicar")
	}

	detalles := make([]Detalle, len(socios))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	for i, socio := range socios {
		wg.Add(1)
		go func(index int, destino Socio) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			detalles[index] = p.enviar(ctx, destino, lote, false)
		}(i, socio)
	}
	wg.Wait()
	resultado.Detalle = detalles
	for _, detalle := range detalles {
		if detalle.Status == "ok" {
			resultado.Procesados++
		} else {
			resultado.Fallidos++
		}
	}
	sort.Slice(resultado.Detalle, func(i, j int) bool {
		return resultado.Detalle[i].Socio < resultado.Detalle[j].Socio
	})
	if resultado.Procesados == 0 {
		return resultado, errors.New("publisher: fallaron todos los envíos")
	}
	return resultado, nil
}

func (p *Publisher) enviar(ctx context.Context, socio Socio, lote Lote, prueba bool) Detalle {
	png, err := p.renderer.Render(Cartelera{
		Socio:      socio.Nombre,
		ChatID:     socio.ChatID,
		MonedaBase: socio.MonedaBase,
		Monedas:    socio.Monedas,
		Lote:       lote.ID,
		Tasas:      lote.Tasas,
		Prueba:     prueba,
		Generada:   p.now(),
	})
	if err != nil {
		p.log.Error("hooty.publish.render", "socio", socio.Nombre, "chat_id", socio.ChatID, "error", err)
		return Detalle{Socio: socio.Nombre, ChatID: socio.ChatID, Status: "error", Error: err.Error()}
	}
	return p.enviarImagen(ctx, socio, lote.ID, png, prueba)
}

// PublishImages despacha las carteleras que el frontend ya generó. Si el
// cliente no envía ninguna, conserva el fallback de renderizado del servidor.
func (p *Publisher) PublishImages(ctx context.Context, isPrueba bool, envios []Envio) (Result, error) {
	lote, err := p.source.LatestLote(ctx)
	if err != nil {
		return Result{}, err
	}
	resultado := Result{IDTasa: lote.ID, Prueba: isPrueba}

	if isPrueba {
		if p.testChatID == "" {
			return resultado, errors.New("publisher: falta configurar EVOLUTION_TEST_GROUP para publicar en prueba")
		}
		if len(envios) == 0 {
			return p.Publish(ctx, true)
		}
		for _, envio := range envios {
			destino := Socio{Nombre: envio.Socio, ChatID: p.testChatID}
			if socio, err := p.resolverSocio(ctx, envio.Socio); err == nil {
				destino = socio
				destino.ChatID = p.testChatID
			}
			var detalle Detalle
			if len(envio.PNG) > 0 {
				detalle = p.enviarImagen(ctx, destino, lote.ID, envio.PNG, true)
			} else {
				detalle = p.enviar(ctx, destino, lote, true)
			}
			resultado.Detalle = append(resultado.Detalle, detalle)
			if detalle.Status == "ok" {
				resultado.Procesados++
			} else {
				resultado.Fallidos++
			}
		}
		sort.Slice(resultado.Detalle, func(i, j int) bool {
			return resultado.Detalle[i].Socio < resultado.Detalle[j].Socio
		})
		if resultado.Procesados == 0 {
			return resultado, errors.New("publisher: fallaron todos los envíos de prueba")
		}
		return resultado, nil
	}

	if len(envios) == 0 {
		return p.Publish(ctx, false)
	}
	for _, envio := range envios {
		if strings.TrimSpace(envio.ChatID) == "" {
			resultado.Fallidos++
			detalle := Detalle{Socio: envio.Socio, Status: "error", Error: "chat_id vacío"}
			resultado.Detalle = append(resultado.Detalle, detalle)
			p.log.Error("hooty.publish.send", "socio", envio.Socio, "error", detalle.Error)
			continue
		}
		var detalle Detalle
		if len(envio.PNG) > 0 {
			detalle = p.enviarImagen(ctx, Socio{Nombre: envio.Socio, ChatID: envio.ChatID},
				lote.ID, envio.PNG, false)
		} else {
			detalle = p.enviar(ctx, Socio{Nombre: envio.Socio, ChatID: envio.ChatID}, lote, false)
		}
		resultado.Detalle = append(resultado.Detalle, detalle)
		if detalle.Status == "ok" {
			resultado.Procesados++
		} else {
			resultado.Fallidos++
		}
	}
	sort.Slice(resultado.Detalle, func(i, j int) bool {
		return resultado.Detalle[i].Socio < resultado.Detalle[j].Socio
	})
	if resultado.Procesados == 0 {
		return resultado, errors.New("publisher: fallaron todos los envíos")
	}
	return resultado, nil
}

// Preview renderiza una cartelera con el mismo generador oficial, sin enviarla.
// El modo prueba no altera el contenido: solo cambia el destino al publicar.
func (p *Publisher) Preview(ctx context.Context, socioNombre string, _ bool) ([]byte, error) {
	lote, err := p.source.LatestLote(ctx)
	if err != nil {
		return nil, err
	}
	destino, err := p.resolverSocio(ctx, socioNombre)
	if err != nil {
		return nil, err
	}
	return p.renderer.Render(Cartelera{
		Socio:      destino.Nombre,
		ChatID:     destino.ChatID,
		MonedaBase: destino.MonedaBase,
		Monedas:    destino.Monedas,
		Lote:       lote.ID,
		Tasas:      lote.Tasas,
		Generada:   p.now(),
	})
}

// PublishSocio renderiza y despacha la cartelera de un solo socio.
func (p *Publisher) PublishSocio(ctx context.Context, socioNombre string, isPrueba bool) (Result, error) {
	lote, err := p.source.LatestLote(ctx)
	if err != nil {
		return Result{}, err
	}
	socio, err := p.resolverSocio(ctx, socioNombre)
	if err != nil {
		return Result{}, err
	}
	resultado := Result{IDTasa: lote.ID, Prueba: isPrueba}
	destino := socio
	if isPrueba {
		if p.testChatID == "" {
			return resultado, errors.New("publisher: falta configurar EVOLUTION_TEST_GROUP para publicar en prueba")
		}
		destino.ChatID = p.testChatID
	}
	detalle := p.enviar(ctx, destino, lote, isPrueba)
	resultado.Detalle = []Detalle{detalle}
	if detalle.Status == "ok" {
		resultado.Procesados = 1
	} else {
		resultado.Fallidos = 1
		return resultado, fmt.Errorf("publisher: falló el envío de %s: %s", socio.Nombre, detalle.Error)
	}
	return resultado, nil
}

func (p *Publisher) resolverSocio(ctx context.Context, nombre string) (Socio, error) {
	nombre = strings.TrimSpace(nombre)
	if nombre != "" {
		if finder, ok := p.source.(SocioFinder); ok {
			if socio, err := finder.SocioByName(ctx, nombre); err == nil {
				return socio, nil
			}
		}
	}
	socios, err := p.source.ActiveSocios(ctx)
	if err != nil {
		return Socio{}, err
	}
	if len(socios) == 0 {
		return Socio{}, errors.New("publisher: no hay socios activos para la cartelera")
	}
	if nombre == "" {
		return socios[0], nil
	}
	for _, socio := range socios {
		if strings.EqualFold(strings.TrimSpace(socio.Nombre), nombre) {
			return socio, nil
		}
	}
	return Socio{}, fmt.Errorf("publisher: no se encontró el socio %q", nombre)
}

func (p *Publisher) enviarImagen(ctx context.Context, socio Socio, loteID string, png []byte, prueba bool) Detalle {
	detalle := Detalle{Socio: socio.Nombre, ChatID: socio.ChatID, Status: "error"}
	prefijo := "📊 [OFICIAL]"
	if prueba {
		prefijo = "🧪 [PRUEBA]"
	}
	caption := fmt.Sprintf("%s Cartelera *%s* - Lote: %s", prefijo, socio.Nombre, loteID)
	if err := p.sender.Send(ctx, socio.ChatID, png, caption); err != nil {
		detalle.Error = err.Error()
		p.log.Error("hooty.publish.send", "socio", socio.Nombre, "chat_id", socio.ChatID,
			"prueba", prueba, "error", err)
		return detalle
	}
	detalle.Status = "ok"
	p.log.Info("hooty.publish.send", "socio", socio.Nombre, "chat_id", socio.ChatID, "prueba", prueba)
	return detalle
}
