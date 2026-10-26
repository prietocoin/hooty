package ingest_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"hooty/internal/clock"
	"hooty/internal/domain"
	"hooty/internal/ingest"
)

type fakeStore struct {
	registro    domain.Registro
	registroErr error
	registrados []domain.Impacto
	guardados   []domain.Comprobante
	ids         [][]int64
}

func (f *fakeStore) RegistrarImpacto(_ context.Context, impacto domain.Impacto) (domain.Registro, error) {
	f.registrados = append(f.registrados, impacto)
	if f.registroErr != nil {
		return domain.Registro{}, f.registroErr
	}
	return f.registro, nil
}

func (f *fakeStore) GuardarComprobanteYMarcar(_ context.Context, c domain.Comprobante, ids []int64) error {
	f.guardados = append(f.guardados, c)
	f.ids = append(f.ids, append([]int64(nil), ids...))
	return nil
}

type fakeMedia struct {
	calls  int
	bytes  int
	image  []byte
	mime   string
	err    error
	lastID string
}

func (f *fakeMedia) Descargar(_ context.Context, instancia string, key, message json.RawMessage) ([]byte, string, error) {
	f.calls++
	f.lastID = instancia + ":" + string(key) + ":" + string(message)
	if f.err != nil {
		return nil, "", f.err
	}
	f.bytes += len(f.image)
	return f.image, f.mime, nil
}

type fakeImages struct {
	calls int
	key   string
	url   string
	err   error
}

func (f *fakeImages) Subir(_ context.Context, key string, data []byte, _ string) (string, error) {
	f.calls++
	f.key = key
	if f.err != nil {
		return "", f.err
	}
	if f.url == "" {
		return "https://cdn.example/" + key, nil
	}
	return f.url, nil
}

type fakeExtractor struct {
	calls   int
	caption string
	image   []byte
	result  domain.Extraccion
	err     error
}

func (f *fakeExtractor) Extraer(_ context.Context, image []byte, _ string, caption string) (domain.Extraccion, error) {
	f.calls++
	f.caption = caption
	f.image = append([]byte(nil), image...)
	if f.err != nil {
		return domain.Extraccion{}, f.err
	}
	return f.result, nil
}

type fakeLiquidator struct {
	calls int
	input domain.LiquidacionInput
	comp  domain.Comprobante
	err   error
}

func (f *fakeLiquidator) Liquidar(_ context.Context, input domain.LiquidacionInput) (domain.Comprobante, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return domain.Comprobante{}, f.err
	}
	return f.comp, nil
}

var testNow = time.Date(2026, time.October, 25, 9, 30, 0, 0, clock.Caracas())

func testEvent() ingest.Evento {
	return ingest.Evento{
		Instancia: "hooty",
		HashLargo: "8a3ef79788979e878129e00ded237dcd",
		HashCorto: "8A3EF797",
		IDGrupo:   "120363000000000002@g.us",
		Caption:   "segundo grupo",
		Timestamp: 1_700_000_000,
		Key:       json.RawMessage(`{"id":"abc","remoteJid":"120363000000000002@g.us"}`),
		Message:   json.RawMessage(`{"imageMessage":{"caption":"segundo grupo"}}`),
		MimeType:  "image/jpeg",
	}
}

func validSettlement() domain.Comprobante {
	return domain.Comprobante{
		Socio1:   "CHASAN",
		Socio2:   "NELSY",
		Tasa1:    2.5,
		Tasa2:    3.5,
		Monto1:   40,
		Monto2:   28.57,
		ME1:      100,
		ME2:      100,
		LoteTasa: "T052",
	}
}

func newService(t *testing.T, store *fakeStore, media *fakeMedia,
	images *fakeImages, extractor *fakeExtractor, liq *fakeLiquidator) *ingest.Service {
	t.Helper()
	svc, err := ingest.NewService(store, media, images, extractor, liq,
		clock.Fixed{T: testNow}, ingest.Options{ProcessingTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestPrimerImpactoNoDescargaNiProcesaImagen(t *testing.T) {
	t.Parallel()

	store := &fakeStore{registro: domain.Registro{
		Status:  domain.StatusWaitingPair,
		Conteo:  1,
		Impacto: domain.Impacto{ID: 1, HashLargo: "8a3ef79788979e878129e00ded237dcd", IDGrupo: "120363000000000001@g.us", RecibidoEn: testNow},
	}}
	media := &fakeMedia{err: errors.New("no debe descargarse")}
	images := &fakeImages{}
	extractor := &fakeExtractor{err: errors.New("no debe extraerse")}
	liq := &fakeLiquidator{err: errors.New("no debe liquidarse")}
	svc := newService(t, store, media, images, extractor, liq)

	decision, err := svc.Recibir(context.Background(), testEvent())
	if err != nil {
		t.Fatalf("el primer impacto no debe fallar: %v", err)
	}
	if decision.Status != domain.StatusWaitingPair {
		t.Fatalf("estado = %q, se esperaba waiting_pair", decision.Status)
	}
	if media.calls != 0 || media.bytes != 0 {
		t.Fatalf("lazy download violado: llamadas=%d bytes=%d", media.calls, media.bytes)
	}
	if images.calls != 0 || extractor.calls != 0 || liq.calls != 0 {
		t.Fatalf("el primer impacto tocó el pipeline pesado: images=%d extractor=%d liq=%d",
			images.calls, extractor.calls, liq.calls)
	}
	if len(store.guardados) != 0 {
		t.Fatal("el primer impacto no debe escribir comprobantes")
	}
	if len(store.registrados) != 1 {
		t.Fatalf("impactos registrados = %d", len(store.registrados))
	}
	impacto := store.registrados[0]
	if impacto.Procesado || impacto.ParejaID != nil {
		t.Fatalf("el impacto huérfano no debe estar procesado ni emparejado: %+v", impacto)
	}
	if impacto.Caption == "" || impacto.HashLargo == "" {
		t.Fatal("el impacto debe conservar texto y hash")
	}
	if impacto.RecibidoEn.Location() != clock.Caracas() {
		t.Fatalf("recibido_en debe estar en Caracas: %v", impacto.RecibidoEn.Location())
	}
}

func TestDuplicadoNoDescargaImagen(t *testing.T) {
	t.Parallel()

	store := &fakeStore{registro: domain.Registro{Status: domain.StatusDuplicate, Conteo: 1}}
	media := &fakeMedia{err: errors.New("no debe descargarse")}
	images := &fakeImages{}
	extractor := &fakeExtractor{err: errors.New("no debe extraerse")}
	liq := &fakeLiquidator{err: errors.New("no debe liquidarse")}
	svc := newService(t, store, media, images, extractor, liq)

	decision, err := svc.Recibir(context.Background(), testEvent())
	if err != nil {
		t.Fatalf("duplicado no debe fallar: %v", err)
	}
	if decision.Status != domain.StatusDuplicate {
		t.Fatalf("estado = %q", decision.Status)
	}
	if media.calls != 0 || images.calls != 0 || extractor.calls != 0 || liq.calls != 0 {
		t.Fatal("un duplicado no debe disparar descarga ni procesamiento")
	}
}

func TestSegundoGrupoDescargaUnaVezYLiquida(t *testing.T) {
	t.Parallel()

	first := testNow.Add(-5 * time.Minute)
	store := &fakeStore{registro: domain.Registro{
		Status: domain.StatusPairReady,
		Conteo: 2,
		Pair: []domain.Impacto{
			{ID: 1, HashLargo: "8a3ef79788979e878129e00ded237dcd", IDGrupo: "120363000000000001@g.us", Caption: "primer grupo", RecibidoEn: first},
			{ID: 2, HashLargo: "8a3ef79788979e878129e00ded237dcd", IDGrupo: "120363000000000002@g.us", Caption: "segundo grupo", RecibidoEn: testNow},
		},
	}}
	media := &fakeMedia{image: []byte("imagen-jpeg"), mime: "image/jpeg"}
	images := &fakeImages{}
	extractor := &fakeExtractor{result: domain.Extraccion{
		Monto: 100, Moneda: "USD", Banco: "BANCO", Titular: "TITULAR", Referencia: "REF",
	}}
	liq := &fakeLiquidator{comp: validSettlement()}
	svc := newService(t, store, media, images, extractor, liq)

	decision, err := svc.Recibir(context.Background(), testEvent())
	if err != nil {
		t.Fatalf("binomio válido falló: %v", err)
	}
	if decision.Status != domain.StatusProcessed {
		t.Fatalf("estado = %q, se esperaba processed", decision.Status)
	}
	if media.calls != 1 {
		t.Fatalf("descargas = %d, se esperaba exactamente 1", media.calls)
	}
	if extractor.calls != 1 || images.calls != 1 || liq.calls != 1 {
		t.Fatalf("llamadas inesperadas: extractor=%d images=%d liq=%d", extractor.calls, images.calls, liq.calls)
	}
	if extractor.caption != "segundo grupo" {
		t.Fatalf("la IA debe recibir el caption del segundo impacto, obtuvo %q", extractor.caption)
	}
	if images.key != "comprobantes/8a3ef79788979e878129e00ded237dcd.jpg" {
		t.Fatalf("clave R2 = %q", images.key)
	}
	if liq.input.Grupo1 != "120363000000000001@g.us" || liq.input.Grupo2 != "120363000000000002@g.us" {
		t.Fatalf("grupos enviados al liquidador: %+v", liq.input)
	}
	if !liq.input.FechaOperacion.Equal(first) {
		t.Fatalf("fecha_operacion = %v, se esperaba %v", liq.input.FechaOperacion, first)
	}
	if len(store.guardados) != 1 {
		t.Fatalf("comprobantes guardados = %d", len(store.guardados))
	}
	saved := store.guardados[0]
	if saved.HashLargo != "8a3ef79788979e878129e00ded237dcd" || saved.URLImagen == "" {
		t.Fatalf("snapshot incompleto: %+v", saved)
	}
	if saved.Grupo1 == saved.Grupo2 || saved.Socio1 == saved.Socio2 {
		t.Fatalf("snapshot no es un binomio: %+v", saved)
	}
	if len(store.ids) != 1 || len(store.ids[0]) != 2 || store.ids[0][0] != 1 || store.ids[0][1] != 2 {
		t.Fatalf("ids marcados = %+v", store.ids)
	}
}

func TestFalloDeIADejaElParSinGuardar(t *testing.T) {
	t.Parallel()

	store := &fakeStore{registro: domain.Registro{
		Status: domain.StatusPairReady,
		Conteo: 2,
		Pair: []domain.Impacto{
			{ID: 1, HashLargo: "8a3ef79788979e878129e00ded237dcd", IDGrupo: "120363000000000001@g.us", RecibidoEn: testNow.Add(-time.Minute)},
			{ID: 2, HashLargo: "8a3ef79788979e878129e00ded237dcd", IDGrupo: "120363000000000002@g.us", RecibidoEn: testNow},
		},
	}}
	media := &fakeMedia{image: []byte("img")}
	images := &fakeImages{}
	extractor := &fakeExtractor{err: errors.New("gemini caído")}
	liq := &fakeLiquidator{comp: validSettlement()}
	svc := newService(t, store, media, images, extractor, liq)

	_, err := svc.Recibir(context.Background(), testEvent())
	if err == nil {
		t.Fatal("se esperaba error de extracción")
	}
	if len(store.guardados) != 0 || len(store.ids) != 0 {
		t.Fatal("no se debe guardar ni marcar el par si la IA falla")
	}
	if liq.calls != 0 {
		t.Fatal("no se debe liquidar sin extracción")
	}
}

func TestParCuyoSegundoNoCoincideNoDescarga(t *testing.T) {
	t.Parallel()

	store := &fakeStore{registro: domain.Registro{
		Status: domain.StatusPairReady,
		Conteo: 2,
		Pair: []domain.Impacto{
			{ID: 1, IDGrupo: "120363000000000001@g.us", RecibidoEn: testNow},
			{ID: 2, IDGrupo: "120363000000000099@g.us", RecibidoEn: testNow},
		},
	}}
	media := &fakeMedia{image: []byte("img")}
	svc := newService(t, store, media, &fakeImages{}, &fakeExtractor{}, &fakeLiquidator{})

	if _, err := svc.Recibir(context.Background(), testEvent()); err == nil {
		t.Fatal("un par desalineado debe fallar")
	}
	if media.calls != 0 {
		t.Fatal("no se debe descargar una imagen que no corresponde al evento")
	}
}

func TestCalcularLiquidacionCongelaDosLados(t *testing.T) {
	t.Parallel()

	input := domain.LiquidacionInput{
		HashLargo:      "hash",
		HashCorto:      "HASH",
		Instancia:      "hooty",
		Extraccion:     domain.Extraccion{Monto: 100, Moneda: "USD", Banco: "B", Titular: "T", Referencia: "R"},
		URLImagen:      "https://cdn.example/imagen.jpg",
		Grupo1:         "120363000000000001@g.us",
		Grupo2:         "120363000000000002@g.us",
		Caption1:       "uno",
		Caption2:       "dos",
		FechaOperacion: testNow.Add(-time.Minute),
	}
	lote := domain.LoteTasas{ID: "T052", Tasas: map[string]float64{
		"USD": 1,
		"VES": 40,
		"COP": 4000,
	}}
	comp, err := ingest.CalcularLiquidacion(input,
		domain.Perfil{Nombre: "CHASAN", MonedaBase: "VES"},
		domain.Perfil{Nombre: "NELSY", MonedaBase: "COP"},
		lote, testNow)
	if err != nil {
		t.Fatalf("liquidación: %v", err)
	}
	if comp.Tasa1 != 0.025 || comp.Monto1 != 4000 || comp.ME1 != 100 {
		t.Fatalf("lado 1 inesperado: tasa=%v monto=%v me=%v", comp.Tasa1, comp.Monto1, comp.ME1)
	}
	if comp.Tasa2 != 0.00025 || comp.Monto2 != 400000 || comp.ME2 != 100 {
		t.Fatalf("lado 2 inesperado: tasa=%v monto=%v me=%v", comp.Tasa2, comp.Monto2, comp.ME2)
	}
	if comp.LoteTasa != "T052" || comp.Socio1 != "CHASAN" || comp.Socio2 != "NELSY" {
		t.Fatalf("snapshot incompleto: %+v", comp)
	}
}

func TestCalcularLiquidacionRechazaMismoSocio(t *testing.T) {
	t.Parallel()

	input := domain.LiquidacionInput{
		HashLargo:  "hash",
		Extraccion: domain.Extraccion{Monto: 100, Moneda: "USD"},
		URLImagen:  "https://cdn.example/imagen.jpg",
		Grupo1:     "120363000000000001@g.us",
		Grupo2:     "120363000000000002@g.us",
	}
	_, err := ingest.CalcularLiquidacion(input,
		domain.Perfil{Nombre: "CHASAN", MonedaBase: "VES"},
		domain.Perfil{Nombre: " chasan ", MonedaBase: "COP"},
		domain.LoteTasas{ID: "T052", Tasas: map[string]float64{"USD": 1, "VES": 40, "COP": 4000}},
		testNow)
	if err == nil {
		t.Fatal("dos grupos que resuelven al mismo socio deben fallar")
	}
}
