package ingest

import (
	"encoding/json"
	"testing"

	"hooty/internal/domain"
)

func TestSimularLiquidacionIndependienteConSpreadYPolaridad(t *testing.T) {
	t.Parallel()

	perfil1 := domain.Perfil{
		Nombre: "TORRES", MonedaBase: "USD",
		Monedas: json.RawMessage(`{"ARS":{"activo":true,"tipo":"D","polar_D":"+","porcentaje":{"deposito":0,"pago":0}}}`),
	}
	perfil2 := domain.Perfil{
		Nombre: "NELSY", MonedaBase: "USD",
		Monedas: json.RawMessage(`{"ARS":{"activo":true,"tipo":"D","polar_D":"-","porcentaje":{"deposito":10,"pago":0}}}`),
	}
	lote := domain.LoteTasas{ID: "T070", Tasas: map[string]float64{"USD": 1, "ARS": 1663}}

	resultado, err := SimularLiquidacion(SimulacionInput{
		MonedaRecibo: "ARS",
		MontoRecibo:  166300,
		Socio1:       "TORRES",
		Socio2:       "NELSY",
	}, perfil1, perfil2, domain.Perfil{}, lote)
	if err != nil {
		t.Fatalf("simulación válida rechazada: %v", err)
	}
	if resultado.Tasa1 != 1663 || resultado.Monto1 != 100 || resultado.ME1 != 100 {
		t.Fatalf("lado 1 inesperado: %+v", resultado)
	}
	if resultado.Tasa2 != 1829 || resultado.Monto2 != -90.92 || resultado.ME2 != -90.92 {
		t.Fatalf("el lado 2 debe usar su propio spread y polaridad: %+v", resultado)
	}
}

func TestSimularLiquidacionTasaFaltanteSoloAfectaSuLado(t *testing.T) {
	t.Parallel()

	perfil1 := domain.Perfil{
		Nombre: "TORRES", MonedaBase: "USD",
		Monedas: json.RawMessage(`{"ARS":{"activo":true,"tipo":"D","polar_D":"+"}}`),
	}
	perfil2 := domain.Perfil{
		Nombre: "NELSY", MonedaBase: "USD",
		Monedas: json.RawMessage(`{"COP":{"activo":true,"tipo":"D"}}`),
	}
	lote := domain.LoteTasas{ID: "T070", Tasas: map[string]float64{"USD": 1, "ARS": 3145}}

	resultado, err := SimularLiquidacion(SimulacionInput{
		MonedaRecibo: "ARS",
		MontoRecibo:  314500,
		Socio1:       "TORRES",
		Socio2:       "NELSY",
	}, perfil1, perfil2, domain.Perfil{}, lote)
	if err != nil {
		t.Fatalf("simulación con un socio sin la moneda rechazada: %v", err)
	}
	if resultado.Tasa1 != 3145 || resultado.Monto1 != 100 || resultado.ME1 != 100 {
		t.Fatalf("el socio con la moneda debe conservar su tasa: %+v", resultado)
	}
	if resultado.Tasa2 != 0 || resultado.Monto2 != 0 || resultado.ME2 != 0 {
		t.Fatalf("el socio sin la moneda debe devolver cero: %+v", resultado)
	}
}

func TestSimularLiquidacionSinTasaEnLoteDevuelveCero(t *testing.T) {
	t.Parallel()

	perfil1 := domain.Perfil{
		Nombre: "TORRES", MonedaBase: "USD",
		Monedas: json.RawMessage(`{"ARS":{"tipo":"D","polar_D":"+"}}`),
	}
	perfil2 := domain.Perfil{
		Nombre: "NELSY", MonedaBase: "USD",
		Monedas: json.RawMessage(`{"ARS":{"tipo":"D","polar_D":"-"}}`),
	}
	lote := domain.LoteTasas{ID: "T070", Tasas: map[string]float64{"USD": 1}}

	resultado, err := SimularLiquidacion(SimulacionInput{
		MonedaRecibo: "ARS",
		MontoRecibo:  314500,
		Socio1:       "TORRES",
		Socio2:       "NELSY",
	}, perfil1, perfil2, domain.Perfil{}, lote)
	if err != nil {
		t.Fatalf("simulación sin tasa de divisa rechazada: %v", err)
	}
	if resultado.Tasa1 != 0 || resultado.Tasa2 != 0 {
		t.Fatalf("sin tasa en el lote ningún lado debe inventar una: %+v", resultado)
	}
	if resultado.Monto1 != 0 || resultado.Monto2 != 0 || resultado.ME1 != 0 || resultado.ME2 != 0 {
		t.Fatalf("montos deben quedar en cero sin tasa: %+v", resultado)
	}
}

func TestSimularLiquidacionRespetaTasasExplicitas(t *testing.T) {
	t.Parallel()

	perfil1 := domain.Perfil{
		Nombre: "TORRES", MonedaBase: "USD",
		Monedas: json.RawMessage(`{"ARS":{"tipo":"D","polar_D":"+"}}`),
	}
	perfil2 := domain.Perfil{
		Nombre: "NELSY", MonedaBase: "USD",
		Monedas: json.RawMessage(`{"ARS":{"tipo":"D","polar_D":"-"}}`),
	}
	lote := domain.LoteTasas{ID: "T070", Tasas: map[string]float64{"USD": 1}}
	resultado, err := SimularLiquidacion(SimulacionInput{
		MonedaRecibo: "ARS",
		MontoRecibo:  49000,
		Socio1:       "TORRES",
		Socio2:       "NELSY",
		Tasa1:        1629,
		Tasa2:        1596,
	}, perfil1, perfil2, domain.Perfil{}, lote)
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

func TestSimularLiquidacionAplicaHerenciaDeFundda(t *testing.T) {
	t.Parallel()

	fundda := domain.Perfil{
		Nombre: "FUNDDA", MonedaBase: "USD",
		Monedas: json.RawMessage(`{"ARS":{"tipo":"D","polar_D":"+","porcentaje":{"deposito":5}}}`),
	}
	perfil1 := domain.Perfil{
		Nombre: "TORRES", MonedaBase: "USD",
		Monedas: json.RawMessage(`{"ARS":{"tipo":"D","polar_D":"+","porcentaje":{"deposito":0}}}`),
	}
	perfil2 := domain.Perfil{Nombre: "NELSY", MonedaBase: "USD", Herencia: true}
	lote := domain.LoteTasas{ID: "T070", Tasas: map[string]float64{"USD": 1, "ARS": 1663}}

	resultado, err := SimularLiquidacion(SimulacionInput{
		MonedaRecibo: "ARS",
		MontoRecibo:  166300,
		Socio1:       "TORRES",
		Socio2:       "NELSY",
	}, perfil1, perfil2, fundda, lote)
	if err != nil {
		t.Fatalf("simulación con herencia rechazada: %v", err)
	}
	if resultado.Tasa2 != 1746 {
		t.Fatalf("el lado con herencia debe usar el spread de FUNDDA: %+v", resultado)
	}
	if resultado.Monto2 >= 0 {
		t.Fatalf("la herencia debe invertir el lado heredado: %+v", resultado)
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
	}, perfil, perfil, domain.Perfil{}, lote)
	if err == nil {
		t.Fatal("el mismo socio debe rechazarse")
	}
}
