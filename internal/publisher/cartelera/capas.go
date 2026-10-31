package cartelera

import (
	"math"

	"github.com/fogleman/gg"
)

// pintarFondo dibuja el fondo a su tamaño natural, anclado arriba a la izquierda
// y recortado por el lienzo.
//
// Esa es la geometría que produce el pipeline original: la plantilla declara
// `background-size: cover`, pero el SVG que genera Satori acaba pintando la
// imagen a su tamaño intrínseco. Como el PNG publicado es la referencia visual,
// se reproduce el resultado y no la intención de la hoja de estilos.
//
// Un fondo ilegible no se propaga como error: la plantilla original caía al azul
// de `backgroundColor` y la cartelera seguía siendo válida.
func pintarFondo(lienzo *gg.Context) {
	lienzo.SetColor(rellenoFondo)
	lienzo.Clear()

	imagen, err := Fondo()
	if err != nil {
		return
	}
	lienzo.DrawImage(imagen, 0, 0)
}

// pintarLogo dibuja el logo del encabezado y devuelve la coordenada donde debe
// empezar el texto. Sin logo, el texto ocupa el margen: la plantilla toleraba su
// ausencia y la cabecera seguía siendo legible.
func pintarLogo(lienzo *gg.Context) float64 {
	imagen, err := Logo()
	if err != nil {
		return margen
	}
	caja := imagen.Bounds()
	ancho := math.Round(altoLogo * float64(caja.Dx()) / float64(caja.Dy()))
	lienzo.DrawImage(escalar(imagen, int(ancho), int(altoLogo)), int(margen), int(margenY))
	return margen + ancho + separacion
}
