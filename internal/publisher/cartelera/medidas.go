package cartelera

import "math"

// Medidas de la cartelera. Son las de la plantilla original: el lienzo mide 1080
// de ancho y la altura crece con cada fila de dos tarjetas.
const (
	anchoLienzo  = 1080
	margen       = 40.0
	altoTarjeta  = 150.0
	anchoTarjeta = 480.0
	yTarjetas    = 130.0
	baseAlto     = 260.0
	pasoFila     = 170.0
	altoMinimo   = 450.0

	// Las tarjetas se dibujan dentro de su caja de borde: el relleno de 20 px y el
	// contorno de 1.5 px van descontados del ancho, como en `box-sizing: border-box`.
	rellenoTarjetaX   = 20.0
	rellenoTarjetaY   = 14.0
	bordeTarjetaAncho = 1.5
)

// filas cuenta las filas de dos tarjetas que necesita la cuadrícula.
func filas(cantidad int) int {
	return (cantidad + 1) / 2
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
