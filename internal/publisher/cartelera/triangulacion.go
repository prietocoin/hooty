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
// Ambos márgenes entran en valor absoluto y se aplican con la misma fórmula del
// motor de comprobantes: compra multiplica por (1 + depósito) y venta por
// (1 - pago). No se divide por (1 + pago) ni se redondea a múltiplos: el cartel
// debe coincidir 1:1 con la tasa asignada al comprobante.
func Prices(cross float64, pct Porcentajes) (buy, sell float64) {
	deposit := math.Abs(pct.Deposito) / 100
	payment := math.Abs(pct.Pago) / 100

	return ApplyRatePrecision(cross * (1 + deposit)),
		ApplyRatePrecision(cross * (1 - payment))
}
