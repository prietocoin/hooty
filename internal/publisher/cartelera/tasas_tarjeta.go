package cartelera

import (
	"math"

	"github.com/fogleman/gg"
)

// Medidas de la franja de tasas: una línea superior, 8 px de aire y dos renglones
// de 12 y 28 px con el interlineado normal de la tipografía.
const altoTasas = 1 + 8 + 12*1.21 + 28*1.21

// pintarTasasTarjeta dibuja la línea separadora, las etiquetas y los dos valores.
//
// Las columnas se apoyan en los extremos del contenido y el separador se calcula
// con el ancho real de cada una: el original usaba `space-between`, así que la
// línea no cae en el centro geométrico sino en el hueco que queda entre ambas.
func pintarTasasTarjeta(lienzo *gg.Context, x, y float64, datos Tarjeta) error {
	izquierda := x + bordeTarjetaAncho + rellenoTarjetaX
	derecha := x + anchoTarjeta - bordeTarjetaAncho - rellenoTarjetaX
	arriba := y + altoTarjeta - bordeTarjetaAncho - rellenoTarjetaY - altoTasas

	lienzo.SetColor(tenue)
	lienzo.SetLineWidth(1)
	lienzo.MoveTo(izquierda, arriba)
	lienzo.LineTo(derecha, arriba)
	lienzo.Stroke()

	compra := textoDeTasa(datos.Compra)
	venta := textoDeTasa(datos.Venta)

	if err := fuente(lienzo, 12); err != nil {
		return err
	}
	lineaEtiqueta := arriba + 1 + 8 + 12*1.21/2
	etiquetaCompra := escribir(lienzo, "COMPRAR", gris, izquierda, lineaEtiqueta)
	etiquetaVenta := escribirDerecha(lienzo, "VENDER", gris, derecha, lineaEtiqueta)

	if err := fuente(lienzo, 28); err != nil {
		return err
	}
	lineaValor := arriba + 1 + 8 + 12*1.21 + 28*1.21/2
	anchoCompra := escribir(lienzo, compra, verde, izquierda, lineaValor)
	anchoVenta := escribirDerecha(lienzo, venta, rojo, derecha, lineaValor)

	columnaIzquierda := math.Max(etiquetaCompra, anchoCompra)
	columnaDerecha := math.Max(etiquetaVenta, anchoVenta)
	libre := math.Max((derecha-izquierda)-columnaIzquierda-columnaDerecha-1, 0)
	separadorX := izquierda + columnaIzquierda + libre/2
	centro := arriba + altoTasas/2

	lienzo.SetColor(separador)
	lienzo.SetLineWidth(1)
	lienzo.MoveTo(separadorX, centro-17.5)
	lienzo.LineTo(separadorX, centro+17.5)
	lienzo.Stroke()
	return nil
}
