package cartelera

import "github.com/fogleman/gg"

// posicionTarjeta devuelve la esquina de la tarjeta en la cuadrícula de dos
// columnas. El hueco horizontal es el margen: con dos tarjetas de 480 sobre un
// ancho útil de 1000 quedan 40 px entre ellas, igual que en la plantilla.
func posicionTarjeta(indice int) (float64, float64) {
	columna := float64(indice % 2)
	fila := float64(indice / 2)
	return margen + columna*(anchoTarjeta+margen), yTarjetas + fila*pasoFila
}

// pintarTarjetas recorre las divisas del socio y dibuja una tarjeta por cada una.
// El orden es el de la cartelera calculada, que ya viene ordenado por país.
func pintarTarjetas(lienzo *gg.Context, tarjetas []Tarjeta) error {
	for indice, datos := range tarjetas {
		x, y := posicionTarjeta(indice)
		if err := pintarTarjeta(lienzo, x, y, datos); err != nil {
			return err
		}
	}
	return nil
}

// pintarTarjeta dibuja el marco y su contenido: el encabezado con la bandera, el
// país y la tendencia, y la franja de tasas separada por una línea tenue.
func pintarTarjeta(lienzo *gg.Context, x, y float64, datos Tarjeta) error {
	tarjeta(lienzo, x, y)
	if err := pintarEncabezadoTarjeta(lienzo, x, y, datos); err != nil {
		return err
	}
	return pintarTasasTarjeta(lienzo, x, y, datos)
}
