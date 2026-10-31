package cartelera

import "github.com/fogleman/gg"

// pintarTasasTarjeta dibuja COMPRAR y VENDER en el extremo derecho de la misma
// fila horizontal, con una línea vertical tenue entre ambas columnas.
func pintarTasasTarjeta(lienzo *gg.Context, x, y float64, datos Tarjeta) error {
	derecha := x + anchoTarjeta - bordeTarjetaAncho - rellenoTarjetaX
	centro := y + altoTarjeta/2
	compra := textoDeTasa(datos.Compra)
	venta := textoDeTasa(datos.Venta)

	if err := fuente(lienzo, 20); err != nil {
		return err
	}
	columnaVenta := maxFloat(68, ancho(lienzo, venta))
	ventaDerecha := derecha
	compraDerecha := derecha - columnaVenta - 26
	separadorX := derecha - columnaVenta - 13

	if err := fuente(lienzo, 9); err != nil {
		return err
	}
	lineaEtiqueta := centro - 11
	escribirDerecha(lienzo, "COMPRAR", gris, compraDerecha, lineaEtiqueta)
	escribirDerecha(lienzo, "VENDER", gris, ventaDerecha, lineaEtiqueta)

	if err := fuente(lienzo, 20); err != nil {
		return err
	}
	lineaValor := centro + 11
	escribirDerecha(lienzo, compra, verde, compraDerecha, lineaValor)
	escribirDerecha(lienzo, venta, rojo, ventaDerecha, lineaValor)

	lienzo.SetColor(separador)
	lienzo.SetLineWidth(1)
	lienzo.MoveTo(separadorX, centro-18)
	lienzo.LineTo(separadorX, centro+18)
	lienzo.Stroke()
	return nil
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
