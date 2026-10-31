package domain

import (
	"math"
	"strings"
	"testing"
	"time"
)

func validComprobante() Comprobante {
	return Comprobante{
		HashLargo:      "8a3ef79788979e878129e00ded237dcd",
		HashCorto:      "8A3EF797",
		Instancia:      "hooty",
		MontoRecibo:    100,
		MonedaRecibo:   "USD",
		URLImagen:      "https://cdn.example/comprobantes/8a3ef797.jpg",
		Grupo1:         "120363000000000001@g.us",
		Grupo2:         "120363000000000002@g.us",
		Socio1:         "CHASAN",
		Socio2:         "NELSY",
		Tasa1:          40,
		Tasa2:          41,
		Monto1:         4000,
		Monto2:         4100,
		ME1:            100,
		ME2:            100,
		LoteTasa:       "T052",
		FechaOperacion: time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC),
		CreadoEn:       time.Date(2026, 10, 25, 10, 1, 0, 0, time.UTC),
	}
}

func TestComprobanteValidateAceptaBinomioReal(t *testing.T) {
	t.Parallel()

	if err := validComprobante().Validate(); err != nil {
		t.Fatalf("un binomio válido no debe fallar: %v", err)
	}
}

func TestComprobanteValidateAceptaTasaCero(t *testing.T) {
	t.Parallel()

	c := validComprobante()
	c.Tasa2 = 0
	c.Monto2 = 0
	c.ME2 = 0
	if err := c.Validate(); err != nil {
		t.Fatalf("una tasa 0 con su lado en cero debe ser válida: %v", err)
	}
}

func TestComprobanteValidateRechazaViolaciones(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(*Comprobante)
		hint   string
	}{
		{"grupos iguales", func(c *Comprobante) { c.Grupo2 = c.Grupo1 }, "distintos"},
		{"socios iguales", func(c *Comprobante) { c.Socio2 = " chasan " }, "distintos"},
		{"grupo vacío", func(c *Comprobante) { c.Grupo1 = "" }, "grupos reales"},
		{"socio general", func(c *Comprobante) { c.Socio2 = "GENERAL" }, "socios reales"},
		{"tasa negativa", func(c *Comprobante) { c.Tasa2 = -1 }, "negativas"},
		{"tipo_op inválido", func(c *Comprobante) { c.TipoOp = "X" }, "tipo_op"},
		{"monto cero", func(c *Comprobante) { c.Monto1 = 0 }, "importe"},
		{"me cero", func(c *Comprobante) { c.ME2 = 0 }, "equivalentes"},
		{"sin lote", func(c *Comprobante) { c.LoteTasa = "" }, "lote_tasa"},
		{"sin imagen", func(c *Comprobante) { c.URLImagen = "" }, "url_imagen"},
		{"sin fecha", func(c *Comprobante) { c.FechaOperacion = time.Time{} }, "fecha_operacion"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validComprobante()
			tc.mutate(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.hint) {
				t.Fatalf("se esperaba error con %q, obtuve %v", tc.hint, err)
			}
		})
	}
}

func TestCanonicalIgnoraAcentosYMayusculas(t *testing.T) {
	t.Parallel()

	if Canonical("  José   Pérez ") != Canonical("JOSE PEREZ") {
		t.Fatal("Canonical debe ignorar acentos, mayúsculas y espacios")
	}
}

func TestNormalizeComprobanteNoAceptaMontoNegativo(t *testing.T) {
	t.Parallel()

	c := NormalizeComprobante(Comprobante{MontoRecibo: -12.345, HashLargo: "ABC"})
	if c.MontoRecibo != 12.34 {
		t.Fatalf("monto normalizado = %v, se esperaba 12.34", c.MontoRecibo)
	}
	if c.HashCorto != "ABC" {
		t.Fatalf("hash corto = %q", c.HashCorto)
	}
}

func TestTruncar2EnterosExactos(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   float64
		want float64
	}{
		{"centavos exactos", 1.15, 1.15},
		{"cero veintinueve", 0.29, 0.29},
		{"tres decimales", 2.675, 2.67},
		{"milésima exacta", 1.005, 1.00},
		{"negativo", -1.15, -1.15},
		{"monto grande", 1234.5678, 1234.56},
		{"menor a un centavo", 0.009, 0},
		{"sin decimales", 42, 42},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Truncar2(tc.in); got != tc.want {
				t.Fatalf("Truncar2(%v) = %v, se esperaba %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestTruncar6Tasas(t *testing.T) {
	t.Parallel()

	if got := Truncar6(1.2345678); got != 1.234567 {
		t.Fatalf("Truncar6 = %v, se esperaba 1.234567", got)
	}
	if got := Truncar6(0.025); got != 0.025 {
		t.Fatalf("Truncar6(0.025) = %v", got)
	}
}

func TestTruncarDecimalesNoFinitos(t *testing.T) {
	t.Parallel()

	if got := Truncar2(math.NaN()); got != 0 {
		t.Fatalf("NaN = %v", got)
	}
	if got := Truncar2(math.Inf(1)); got != 0 {
		t.Fatalf("Inf = %v", got)
	}
}

func TestExtraccionValidate(t *testing.T) {
	t.Parallel()

	if err := (Extraccion{Monto: 10, Moneda: "usd"}).Validate(); err != nil {
		t.Fatalf("extracción válida rechazada: %v", err)
	}
	if err := (Extraccion{Monto: 0, Moneda: "USD"}).Validate(); err == nil {
		t.Fatal("monto cero debe fallar")
	}
	if err := (Extraccion{Monto: 10, Moneda: " "}).Validate(); err == nil {
		t.Fatal("moneda vacía debe fallar")
	}
}
