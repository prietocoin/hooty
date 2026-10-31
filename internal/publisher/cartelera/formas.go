package cartelera

import (
	"image"
	"image/color"

	"github.com/fogleman/gg"
)

// pildora dibuja una etiqueta redondeada: relleno, contorno y radio en una pieza.
// Es la forma que comparten la insignia OFICIAL, las del pie y la de tendencia.
func pildora(lienzo *gg.Context, x, y, ancho, alto, radio float64, relleno, borde color.Color, grosor float64) {
	lienzo.DrawRoundedRectangle(x, y, ancho, alto, radio)
	lienzo.SetColor(relleno)
	lienzo.FillPreserve()
	lienzo.SetColor(borde)
	lienzo.SetLineWidth(grosor)
	lienzo.Stroke()
}

// tarjeta dibuja el marco de una tarjeta: fondo translúcido, contorno fino y la
// franja azul superior que la plantilla declaraba como borde de 4 px.
func tarjeta(lienzo *gg.Context, x, y float64) {
	marco := func() {
		lienzo.DrawRoundedRectangle(x, y, anchoTarjeta, altoTarjeta, 16)
	}
	marco()
	lienzo.SetColor(rellenoTarjeta)
	lienzo.FillPreserve()
	lienzo.SetColor(bordeTarjeta)
	lienzo.SetLineWidth(bordeTarjetaAncho)
	lienzo.Stroke()

	// La franja se recorta contra el marco para que siga la curva de las esquinas,
	// igual que un border-top con border-radius.
	marco()
	lienzo.Clip()
	lienzo.DrawRectangle(x, y, anchoTarjeta, 4)
	lienzo.SetColor(azul)
	lienzo.Fill()
	lienzo.ResetClip()
}

// bandera dibuja la imagen con esquinas redondeadas y su contorno tenue. El
// recorte usa la máscara del lienzo, que también afecta a las imágenes, de modo
// que el borde no tapa el redondeo.
func bandera(lienzo *gg.Context, imagen image.Image, x, y float64) {
	lienzo.DrawRoundedRectangle(x, y, anchoBandera, altoBandera, 4)
	lienzo.Clip()
	lienzo.DrawImage(imagen, int(x), int(y))
	lienzo.ResetClip()

	lienzo.DrawRoundedRectangle(x, y, anchoBandera, altoBandera, 4)
	lienzo.SetColor(bordeBandera)
	lienzo.SetLineWidth(1)
	lienzo.Stroke()
}
