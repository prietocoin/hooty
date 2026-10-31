package cartelera

import "math"

// Medidas de la cartelera. El lienzo se compactó a 640 px para que la fila
// horizontal no deje un hueco muerto en el centro; la altura crece por cada fila.
const (
	anchoLienzo  = 640
	margen       = 40.0
	altoTarjeta  = 76.0
	anchoTarjeta = 560.0
	yTarjetas    = 130.0
	baseAlto     = 260.0
	pasoFila     = 88.0
	altoMinimo   = 450.0

	// Las tarjetas se dibujan dentro de su caja de borde: el relleno de 20 px y el
	// contorno de 1 px van descontados del ancho, como en `box-sizing: border-box`.
	rellenoTarjetaX   = 20.0
	rellenoTarjetaY   = 12.0
	bordeTarjetaAncho = 1.0
)

// filas cuenta las filas de una tarjeta que necesita la lista vertical.
func filas(cantidad int) int {
	return cantidad
}

// altoDelLienzo reproduce el cálculo del servicio original: una base fija más una
// fila por cada dos tarjetas, con un mínimo para las carteleras cortas.
func altoDelLienzo(cantidad int) int {
	alto := int(math.Round(baseAlto + float64(filas(cantidad))*pasoFila))
	if alto < int(altoMinimo) {
		return int(altoMinimo)
	}
	return alto
}
