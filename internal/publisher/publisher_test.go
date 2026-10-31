package publisher

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
)

type fakeSource struct {
	lote   Lote
	socios []Socio
	err    error
}

func (f fakeSource) LatestLote(context.Context) (Lote, error) {
	return f.lote, f.err
}

func (f fakeSource) ActiveSocios(context.Context) ([]Socio, error) {
	return f.socios, f.err
}

type fakeSender struct {
	mu     sync.Mutex
	calls  []string
	failOn string
}

func (f *fakeSender) Send(_ context.Context, chatID string, png []byte, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, chatID)
	if chatID == f.failOn {
		return errors.New("webhook respondió HTTP 500")
	}
	if len(png) == 0 {
		return errors.New("png vacío")
	}
	return nil
}

func (f *fakeSender) chatIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

type fakeRenderer struct{}

func (fakeRenderer) Render(Cartelera) ([]byte, error) {
	return []byte("png"), nil
}

type captureRenderer struct {
	last Cartelera
}

func (c *captureRenderer) Render(cartelera Cartelera) ([]byte, error) {
	c.last = cartelera
	return []byte("png"), nil
}

func TestPreviewMantieneNombreRealYSpreadsEnModoPrueba(t *testing.T) {
	t.Parallel()

	renderer := &captureRenderer{}
	p := New(fakeSource{
		lote: Lote{ID: "T096", Tasas: map[string]float64{"USD": 1, "ARS": 1622}},
		socios: []Socio{
			{Nombre: "ANA", ChatID: "ana@g.us", MonedaBase: "USD", Monedas: []byte(`{"ARS":{"porcentaje":{"deposito":5,"pago":3}}}`)},
		},
	}, &fakeSender{}, renderer, "prueba@g.us", nil)
	if _, err := p.Preview(context.Background(), "ANA", true); err != nil {
		t.Fatalf("preview rechazada: %v", err)
	}
	if renderer.last.Socio != "ANA" {
		t.Fatalf("el cartel de prueba debe conservar el socio real, obtuve %q", renderer.last.Socio)
	}
	if len(renderer.last.Monedas) == 0 || renderer.last.MonedaBase != "USD" {
		t.Fatalf("faltan spreads o moneda base en el cartel: %+v", renderer.last)
	}
}

func testPublisher(testChat string, sender *fakeSender) *Publisher {
	return New(fakeSource{
		lote: Lote{ID: "T096", Tasas: map[string]float64{"USD": 1, "ARS": 1622}},
		socios: []Socio{
			{Nombre: "ANA", ChatID: "ana@g.us"},
			{Nombre: "BETO", ChatID: "beto@g.us"},
		},
	}, sender, fakeRenderer{}, testChat, nil)
}

func TestPublishPruebaEnviaCartelesIndividualesAlGrupoDePrueba(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	resultado, err := testPublisher("prueba@g.us", sender).Publish(context.Background(), true)
	if err != nil {
		t.Fatalf("publicación de prueba rechazada: %v", err)
	}
	destinos := sender.chatIDs()
	if len(destinos) != 2 {
		t.Fatalf("debe enviar un cartel por socio al grupo test: %v", destinos)
	}
	for _, destino := range destinos {
		if destino != "prueba@g.us" {
			t.Fatalf("destino de prueba incorrecto: %v", destinos)
		}
	}
	if resultado.Procesados != 2 || !resultado.Prueba {
		t.Fatalf("resultado inesperado: %+v", resultado)
	}
}

func TestPublishProduccionEnviaCadaSocioActivo(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	resultado, err := testPublisher("prueba@g.us", sender).Publish(context.Background(), false)
	if err != nil {
		t.Fatalf("publicación de producción rechazada: %v", err)
	}
	destinos := sender.chatIDs()
	sort.Strings(destinos)
	if len(destinos) != 2 || destinos[0] != "ana@g.us" || destinos[1] != "beto@g.us" {
		t.Fatalf("destinos de producción = %v", destinos)
	}
	if resultado.Procesados != 2 || resultado.Fallidos != 0 {
		t.Fatalf("resultado inesperado: %+v", resultado)
	}
}

func TestPublishProduccionRegistraFalloParcial(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{failOn: "beto@g.us"}
	resultado, err := testPublisher("prueba@g.us", sender).Publish(context.Background(), false)
	if err != nil {
		t.Fatalf("un fallo parcial no debe abortar la publicación: %v", err)
	}
	if resultado.Procesados != 1 || resultado.Fallidos != 1 {
		t.Fatalf("resultado inesperado: %+v", resultado)
	}
	var fallido Detalle
	for _, detalle := range resultado.Detalle {
		if detalle.Socio == "BETO" {
			fallido = detalle
		}
	}
	if fallido.Status != "error" || fallido.Error == "" {
		t.Fatalf("detalle del fallo inesperado: %+v", resultado.Detalle)
	}
}

func TestCarteleraRenderizaPNG(t *testing.T) {
	t.Parallel()

	png, err := NewCarteleraRenderer().Render(Cartelera{
		Socio: "ANA", Lote: "T096", Prueba: true,
		Tasas: map[string]float64{"USD": 1, "ARS": 1622, "COP": 3094},
	})
	if err != nil {
		t.Fatalf("render rechazado: %v", err)
	}
	if len(png) < 100 || string(png[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("no se generó un PNG válido: %d bytes", len(png))
	}
}

func TestPublishImagesUsaImagenesDelCliente(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	p := testPublisher("prueba@g.us", sender)
	resultado, err := p.PublishImages(context.Background(), false, []Envio{
		{Socio: "ANA", ChatID: "ana@g.us", PNG: []byte("png-ana")},
		{Socio: "BETO", ChatID: "beto@g.us", PNG: []byte("png-beto")},
	})
	if err != nil {
		t.Fatalf("publicación con imágenes rechazada: %v", err)
	}
	destinos := sender.chatIDs()
	sort.Strings(destinos)
	if len(destinos) != 2 || destinos[0] != "ana@g.us" || destinos[1] != "beto@g.us" {
		t.Fatalf("destinos = %v", destinos)
	}
	if resultado.Procesados != 2 {
		t.Fatalf("resultado inesperado: %+v", resultado)
	}
}

func TestPublishImagesPruebaIgnoraChatsDeSocios(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	p := testPublisher("prueba@g.us", sender)
	resultado, err := p.PublishImages(context.Background(), true, []Envio{
		{Socio: "ANA", ChatID: "ana@g.us", PNG: []byte("png-ana")},
		{Socio: "BETO", ChatID: "beto@g.us", PNG: []byte("png-beto")},
	})
	if err != nil {
		t.Fatalf("publicación de prueba rechazada: %v", err)
	}
	destinos := sender.chatIDs()
	if len(destinos) != 2 {
		t.Fatalf("prueba debe reenviar cada cartel al grupo test: %v", destinos)
	}
	for _, destino := range destinos {
		if destino != "prueba@g.us" {
			t.Fatalf("prueba debe usar solo el grupo test: %v", destinos)
		}
	}
	if resultado.Procesados != 2 || !resultado.Prueba {
		t.Fatalf("resultado inesperado: %+v", resultado)
	}
}

func TestPublishPruebaSinGrupoFalla(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	if _, err := testPublisher("", sender).Publish(context.Background(), true); err == nil {
		t.Fatal("publicar en prueba sin chat de pruebas debe fallar")
	}
	if len(sender.chatIDs()) != 0 {
		t.Fatal("no debe enviarse nada sin grupo de pruebas")
	}
}
