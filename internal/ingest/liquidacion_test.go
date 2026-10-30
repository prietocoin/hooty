package ingest

import (
	"testing"

	"hooty/internal/domain"
)

func TestSimularLiquidacionUsaFormulaCanonica(t *testing.T) {
	t.Parallel()

	perfil1 := domain.Perfil{Nombre: "CHASAN", MonedaBase: "USD"}
	perfil2 := domain.Perfil{Nombre: "NELSY", MonedaBase: "PEN"}
	lote := domain.LoteTasas{ID: "T070", Tasas: map[string]float64{
		"USD": 1, "PEN": 3.7,
		"USD_1": 1, "USD_2": 3.7,
	}}

	resultado, err := SimularLiquidacion(SimulacionInput{
		MonedaRecibo: "USD",
		MontoRecibo:  100,
		Socio1:       "CHASAN",
		Socio2:       "NELSY",
		LoteTasa:     "T070",
	}, perfil1, perfil2, lote)
	if err != nil {
		t.Fatalf("simulación válida rechazada: %v", err)
	}
	if resultado.Tasa1 != 1 || resultado.Monto1 != 100 || resultado.ME1 != 100 {
		t.Fatalf("lado 1 inesperado: %+v", resultado)
	}
	if resultado.Tasa2 != 3.7 || resultado.Monto2 != -27.02 || resultado.ME2 != -7.30 {
		t.Fatalf("lado 2 inesperado: %+v", resultado)
	}
}

func TestSimularLiquidacionTasaFaltanteDevuelveCero(t *testing.T) {
	t.Parallel()

	perfil1 := domain.Perfil{Nombre: "TORRES", MonedaBase: "USD"}
	perfil2 := domain.Perfil{Nombre: "NELSY", MonedaBase: "USD"}
	lote := domain.LoteTasas{ID: "T070", Tasas: map[string]float64{
		"USD":   1,
		"ARS_1": 3145,
	}}

	resultado, err := SimularLiquidacion(SimulacionInput{
		MonedaRecibo: "ARS",
		MontoRecibo:  314500,
		Socio1:       "TORRES",
		Socio2:       "NELSY",
	}, perfil1, perfil2, lote)
	if err != nil {
		t.Fatalf("simulación con tasa faltante rechazada: %v", err)
	}
	if resultado.Tasa1 != 3145 || resultado.Monto1 != 100 || resultado.ME1 != 100 {
		t.Fatalf("lado 1 inesperado: %+v", resultado)
	}
	if resultado.Tasa2 != 0 || resultado.Monto2 != 0 || resultado.ME2 != 0 {
		t.Fatalf("el lado sin tasa debe devolver cero, no copiar al otro socio: %+v", resultado)
	}
}

func TestSimularLiquidacionSinTasasDeLadoDevuelveCero(t *testing.T) {
	t.Parallel()

	perfil1 := domain.Perfil{Nombre: "TORRES", MonedaBase: "USD"}
	perfil2 := domain.Perfil{Nombre: "NELSY", MonedaBase: "USD"}
	lote := domain.LoteTasas{ID: "T070", Tasas: map[string]float64{"USD": 1, "ARS": 3145}}

	resultado, err := SimularLiquidacion(SimulacionInput{
		MonedaRecibo: "ARS",
		MontoRecibo:  314500,
		Socio1:       "TORRES",
		Socio2:       "NELSY",
	}, perfil1, perfil2, lote)
	if err != nil {
		t.Fatalf("simulación sin tasas de lado rechazada: %v", err)
	}
	if resultado.Tasa1 != 0 || resultado.Tasa2 != 0 {
		t.Fatalf("no debe usarse la tasa genérica como fallback: %+v", resultado)
	}
	if resultado.Monto1 != 0 || resultado.Monto2 != 0 || resultado.ME1 != 0 || resultado.ME2 != 0 {
		t.Fatalf("montos deben quedar en cero sin tasa: %+v", resultado)
	}
}

func TestSimularLiquidacionRespetaTasasExplicitas(t *testing.T) {
	t.Parallel()

	perfil1 := domain.Perfil{Nombre: "TORRES", MonedaBase: "USD"}
	perfil2 := domain.Perfil{Nombre: "NELSY", MonedaBase: "USD"}
	lote := domain.LoteTasas{ID: "T070", Tasas: map[string]float64{"USD": 1}}
	resultado, err := SimularLiquidacion(SimulacionInput{
		MonedaRecibo: "ARS",
		MontoRecibo:  49000,
		Socio1:       "TORRES",
		Socio2:       "NELSY",
		Tasa1:        1629,
		Tasa2:        1596,
	}, perfil1, perfil2, lote)
	if err != nil {
		t.Fatalf("simulación con tasas explícitas rechazada: %v", err)
	}
	if resultado.Tasa1 != 1629 || resultado.Tasa2 != 1596 {
		t.Fatalf("tasas explícitas no respetadas: %+v", resultado)
	}
	if resultado.Monto1 != 30.07 || resultado.Monto2 != -30.70 {
		t.Fatalf("montos inesperados: %+v", resultado)
	}
}

func TestSimularLiquidacionRechazaMismoSocio(t *testing.T) {
	t.Parallel()

	perfil := domain.Perfil{Nombre: "CHASAN", MonedaBase: "USD"}
	lote := domain.LoteTasas{ID: "T070", Tasas: map[string]float64{"USD": 1}}
	_, err := SimularLiquidacion(SimulacionInput{
		MonedaRecibo: "USD",
		MontoRecibo:  100,
		Socio1:       "CHASAN",
		Socio2:       "chasan",
	}, perfil, perfil, lote)
	if err == nil {
		t.Fatal("el mismo socio debe rechazarse")
	}
}
