package cartelera

import "testing"

// TestPricesVentaTruncaSinRedondearAMultiplosFija el caso del lote T096 (COP):
// base 3065 con 3% por lado. La venta debe ser 3065*(1-0.03) = 2973.05,
// truncada a 2973, nunca 2975.
func TestPricesVentaTruncaSinRedondearAMultiplosFija(t *testing.T) {
	t.Parallel()

	buy, sell := Prices(3065, Porcentajes{Deposito: 3, Pago: 3})
	if buy != 3156 {
		t.Fatalf("compra = %v, se esperaba 3156", buy)
	}
	if sell != 2973 {
		t.Fatalf("venta = %v, se esperaba 2973 (sin redondeo a múltiplos de 5)", sell)
	}
}

func TestPricesUsaMargenAbsolutoEnAmbosLados(t *testing.T) {
	t.Parallel()

	buyNeg, sellNeg := Prices(3065, Porcentajes{Deposito: -3, Pago: -3})
	buy, sell := Prices(3065, Porcentajes{Deposito: 3, Pago: 3})
	if buyNeg != buy || sellNeg != sell {
		t.Fatalf("el signo del margen no debe alterar el precio: neg=(%v,%v) pos=(%v,%v)",
			buyNeg, sellNeg, buy, sell)
	}
}

func TestCardForCOPVentaCoincideConComprobante(t *testing.T) {
	t.Parallel()

	cfg := &Moneda{Porcentaje: Porcentajes{Deposito: 3, Pago: 3}}
	actual := Lote{Tasas: map[string]any{"COP": 3065}}
	tarjeta, ok := cardFor("COP", cfg, actual, Lote{}, 1, 1)
	if !ok {
		t.Fatal("cardFor descartó COP")
	}
	if tarjeta.Compra != 3156 || tarjeta.Venta != 2973 {
		t.Fatalf("COP compra/venta = %v/%v, se esperaba 3156/2973",
			tarjeta.Compra, tarjeta.Venta)
	}
}
