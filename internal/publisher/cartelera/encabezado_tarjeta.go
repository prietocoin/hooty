package cartelera

import "github.com/fogleman/gg"

// Medidas del encabezado de una tarjeta. El centro de la línea se mide desde el
// borde superior: 4 px de franja, 14 de relleno y media caja de texto de 24 px.
const (
	centroEncabezado  = 4 + 14 + 24*1.21/2
	separacionBandera = 12.0
)

// pintarEncabezadoTarjeta dibuja la bandera, el nombre del país y la insignia de
// tendencia, alineados como la fila superior de la plantilla.
func pintarEncabezadoTarjeta(lienzo *gg.Context, x, y float64, datos Tarjeta) error {
	centro := y + centroEncabezado
	textoX := x + bordeTarjetaAncho + rellenoTarjetaX
	if imagen, ok := Bandera(datos.Bandera); ok {
		bandera(lienzo, recortarComoCover(imagen, int(anchoBandera), int(altoBandera)), textoX, centro-altoBandera/2)
		textoX += anchoBandera + separacionBandera
	}

	if err := fuente(lienzo, 24); err != nil {
		return err
	}
	escribir(lienzo, datos.NombrePais, blanco, textoX, centro)

	// La insignia se pega al borde derecho del contenido, como el `space-between`.
	derecha := x + anchoTarjeta - bordeTarjetaAncho - rellenoTarjetaX
	return pintarTendencia(lienzo, derecha-anchoInsignia, centro-altoInsignia/2, datos.Trend)
}
