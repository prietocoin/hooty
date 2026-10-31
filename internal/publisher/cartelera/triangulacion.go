package cartelera

import (
	"math"
)

// CrossRate expresa la cotización de una divisa en términos de la moneda nativa
// del socio.
//
// La división es lo que hace que la cartelera sea "triangulada": el socio ve
// cuántas unidades de su propia moneda cuesta cada divisa, y no la cotización
// cruda del lote, que está expresada en otra referencia.
func CrossRate(rate, base float64) float64 {
	return rate / base
}

// Prices aplica los márgenes del socio sobre una tasa cruzada y devuelve el par
// compra/venta ya normalizado por la regla de precisión.
//
// El margen de depósito multiplica y el de pago divide, y ambos entran en valor
// absoluto: un socio que escriba -5 en depósito recibe exactamente el mismo
// resultado que con 5. El orden de las operaciones se conserva del original al
// detalle, porque la precisión de la coma flotante forma parte de la salida.
func Prices(cross float64, pct Porcentajes) (buy, sell float64) {
	deposit := math.Abs(pct.Deposito) / 100
	payment := math.Abs(pct.Pago) / 100

	return ApplyRatePrecision(cross * (1 + deposit)),
		ApplyRatePrecision(cross / (1 + payment))
}
