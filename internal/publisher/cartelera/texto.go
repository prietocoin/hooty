package cartelera

import (
	"image/color"

	"github.com/fogleman/gg"
)

// Inter declara su ascender y su descender sobre 2048 unidades de em. Son los
// mismos números con los que el navegador arma la caja de línea, y hacen falta
// porque gg centra el texto por la mitad del tamaño y no por esa caja.
const (
	ascensoInter  = 1984.0 / 2048
	descensoInter = 494.0 / 2048
)

// baseDeLinea devuelve la línea base que deja el texto centrado en y igual que la
// hoja de estilos: la caja de línea va del ascender al descender, así que su
// centro queda a media diferencia por debajo de la línea base.
//
// Sin esta corrección un valor de 28 px se dibuja unos 4 px más abajo que en la
// cartelera publicada, y el desfase crece con el tamaño de la letra.
func baseDeLinea(lienzo *gg.Context, y float64) float64 {
	return y + (ascensoInter-descensoInter)*lienzo.FontHeight()/2
}

// fuente fija la tipografía al tamaño pedido. gg no redimensiona una cara ya
// cargada, así que cada tamaño crea la suya; el parseo de la fuente, que es lo
// caro, ya está cacheado en assets.go.
func fuente(lienzo *gg.Context, tamano float64) error {
	cara, err := Letra(tamano)
	if err != nil {
		return err
	}
	lienzo.SetFontFace(cara)
	return nil
}

// ancho mide el texto con la tipografía que el lienzo tiene fijada. Los helpers
// de abajo suponen que la cara ya se fijó con fuente().
func ancho(lienzo *gg.Context, texto string) float64 {
	medido, _ := lienzo.MeasureString(texto)
	return medido
}

// escribir dibuja el texto con su borde izquierdo en x y centrado en y, y devuelve
// el ancho para poder encadenar segmentos de colores distintos en una misma línea.
func escribir(lienzo *gg.Context, texto string, tono color.Color, x, y float64) float64 {
	lienzo.SetColor(tono)
	lienzo.DrawStringAnchored(texto, x, baseDeLinea(lienzo, y), 0, 0)
	return ancho(lienzo, texto)
}

// escribirDerecha alinea el texto por su borde derecho.
func escribirDerecha(lienzo *gg.Context, texto string, tono color.Color, x, y float64) float64 {
	lienzo.SetColor(tono)
	lienzo.DrawStringAnchored(texto, x, baseDeLinea(lienzo, y), 1, 0)
	return ancho(lienzo, texto)
}
