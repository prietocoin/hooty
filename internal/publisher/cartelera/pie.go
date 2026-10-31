package cartelera

import "github.com/fogleman/gg"

// Medidas del pie: píldoras con 15 px de texto y 6 de relleno vertical, apoyadas
// en el margen inferior del lienzo.
const (
	altoPie      = 6 + 6 + 15*1.21
	radioPildora = 20.0
)

// pintarPie dibuja las dos píldoras adheridas al borde inferior: la de la marca y
// la del lote con la fecha de actualización.
func pintarPie(lienzo *gg.Context, datos Payload) error {
	if err := fuente(lienzo, 15); err != nil {
		return err
	}
	y := float64(lienzo.Height()) - margen - altoPie
	pintarMarca(lienzo, margen, y)
	pintarLote(lienzo, anchoLienzo-margen, y, datos)
	return nil
}

// pintarMarca dibuja la píldora de la izquierda: motor, viñeta y casa.
func pintarMarca(lienzo *gg.Context, x, y float64) {
	contenido := ancho(lienzo, "TasasHub Engine") + 8 + ancho(lienzo, "•") + 8 + ancho(lienzo, "Fundablock")
	anchoPildora := contenido + 32
	pildora(lienzo, x, y, anchoPildora, altoPie, radioPildora, tintePie, bordePie, 1)

	centro := y + altoPie/2
	pen := x + 16
	pen += escribir(lienzo, "TasasHub Engine", blanco, pen, centro) + 8
	pen += escribir(lienzo, "•", azul, pen, centro) + 8
	escribir(lienzo, "Fundablock", gris, pen, centro)
}

// pintarLote dibuja la píldora de la derecha, alineada por su borde derecho.
func pintarLote(lienzo *gg.Context, derecha, y float64, datos Payload) {
	lote := "Lote: " + datos.LoteTasa
	fecha := datos.Fecha + " • " + datos.Hora
	contenido := ancho(lienzo, lote) + 10 + ancho(lienzo, "|") + 10 + ancho(lienzo, fecha)
	anchoPildora := contenido + 36
	x := derecha - anchoPildora
	pildora(lienzo, x, y, anchoPildora, altoPie, radioPildora, tintePieAlto, bordePieAzul, 1)

	centro := y + altoPie/2
	pen := x + 18
	pen += escribir(lienzo, lote, ambar, pen, centro) + 10
	pen += escribir(lienzo, "|", azul, pen, centro) + 10
	escribir(lienzo, fecha, textoPie, pen, centro)
}
