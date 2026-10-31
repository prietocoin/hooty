package cartelera

import (
	"image/color"

	"github.com/fogleman/gg"
)

// espaciadoLetras reproduce el `letterSpacing: 1px` de la plantilla. Solo lo
// declaran los rótulos FUNDABLOCK y OFICIAL.
const espaciadoLetras = 1.0

// anchoEspaciado mide el texto como lo dibuja escribirEspaciado: letra a letra más
// el espaciado. Se usa para dimensionar la píldora OFICIAL con el ancho real del
// rótulo y no con una estimación de un píxel por letra.
func anchoEspaciado(lienzo *gg.Context, texto string) float64 {
	total := 0.0
	for _, letra := range texto {
		total += ancho(lienzo, string(letra)) + espaciadoLetras
	}
	return total
}

// escribirEspaciado dibuja el texto letra a letra con un píxel extra entre ellas,
// porque gg no tiene espaciado de letras. Sin él, los dos rótulos quedarían unos
// 10 px más estrechos que en la cartelera publicada y arrastrarían al resto de la
// línea.
//
// Cada letra se mide y se avanza por separado: se pierde el kerning entre pares,
// que en mayúsculas de Inter no llega al píxel.
func escribirEspaciado(lienzo *gg.Context, texto string, tono color.Color, x, y float64) float64 {
	lienzo.SetColor(tono)
	base := baseDeLinea(lienzo, y)
	avance := 0.0
	for _, letra := range texto {
		caracter := string(letra)
		lienzo.DrawStringAnchored(caracter, x+avance, base, 0, 0)
		avance += ancho(lienzo, caracter) + espaciadoLetras
	}
	return avance
}
