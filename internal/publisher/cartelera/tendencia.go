package cartelera

import (
	"image/color"

	"github.com/fogleman/gg"
)

// Medidas de la insignia: 18 px de icono con 4 de relleno vertical y 8 horizontal,
// según la plantilla. El icono vive en un viewBox de 24, de ahí la escala.
const (
	altoInsignia  = 26.0
	anchoInsignia = 34.0
	escalaIcono   = 18.0 / 24.0
)

// pintarTendencia dibuja la insignia y la flecha de la tendencia. La flecha se
// traza con las mismas polilíneas del SVG original, escaladas a 18 px.
func pintarTendencia(lienzo *gg.Context, x, y float64, tendencia string) error {
	relleno, borde, trazo := tonosTendencia(tendencia)
	pildora(lienzo, x, y, anchoInsignia, altoInsignia, 8, relleno, borde, 1)

	lienzo.SetColor(trazo)
	lienzo.SetLineWidth(3 * escalaIcono)
	lienzo.SetLineCapRound()
	lienzo.SetLineJoinRound()
	for _, polilinea := range trazos(tendencia) {
		for indice, punto := range polilinea {
			px := x + 8 + punto.x*escalaIcono
			py := y + 4 + punto.y*escalaIcono
			if indice == 0 {
				lienzo.MoveTo(px, py)
			} else {
				lienzo.LineTo(px, py)
			}
		}
		lienzo.Stroke()
	}
	return nil
}

// tonosTendencia devuelve relleno, contorno y trazo de cada estado. El estado
// desconocido cae en "equal", igual que el `|| 'equal'` del servicio original.
func tonosTendencia(tendencia string) (color.Color, color.Color, color.Color) {
	switch tendencia {
	case "up":
		return tinteVerde, bordeVerde, verde
	case "down":
		return tinteRojo, bordeRojo, rojo
	default:
		return tinteAmbar, bordeAmbar, ambar
	}
}
