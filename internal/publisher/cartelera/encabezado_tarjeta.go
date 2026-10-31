package cartelera

import "github.com/fogleman/gg"

// Medidas del encabezado de una tarjeta horizontal.
const (
	centroEncabezado  = altoTarjeta / 2
	separacionBandera = 10.0
	separacionTexto   = 8.0
)

// pintarEncabezadoTarjeta dibuja bandera, país, siglas y tendencia en una sola
// línea horizontal, alineados al centro vertical de la fila.
func pintarEncabezadoTarjeta(lienzo *gg.Context, x, y float64, datos Tarjeta) error {
	centro := y + centroEncabezado
	textoX := x + bordeTarjetaAncho + rellenoTarjetaX
	if imagen, ok := Bandera(datos.Bandera); ok {
		bandera(lienzo, recortarComoCover(imagen, int(anchoBandera), int(altoBandera)),
			textoX, centro-altoBandera/2)
		textoX += anchoBandera + separacionBandera
	}

	if err := fuente(lienzo, 22); err != nil {
		return err
	}
	textoX += escribir(lienzo, datos.NombrePais, blanco, textoX, centro) + separacionTexto

	if err := fuente(lienzo, 12); err != nil {
		return err
	}
	textoX += escribir(lienzo, datos.Code, cian, textoX, centro) + separacionTexto

	return pintarTendencia(lienzo, textoX, centro-altoInsignia/2, datos.Trend)
}
