package cartelera

import (
	"fmt"
	"image"

	"golang.org/x/image/draw"
)

// Medidas del hueco de la bandera en el encabezado de cada tarjeta: 32x32 con
// radio 4, según la estructura horizontal solicitada.
const (
	anchoBandera = 32.0
	altoBandera  = 32.0
)

// Bandera devuelve la bandera del código ISO recibido. Un código desconocido no es
// un error: la plantilla original omitía la imagen y la tarjeta seguía saliendo,
// así que aquí se responde "no hay" en lugar de romper la cartelera completa.
func Bandera(iso string) (image.Image, bool) {
	if iso == "" {
		return nil, false
	}
	imagen, err := decodificar(fmt.Sprintf("assets/flags/%s.png", iso))
	if err != nil {
		return nil, false
	}
	return imagen, true
}

// recortarComoCover imita `object-fit: cover`: escala la imagen hasta cubrir el
// recuadro y recorta el centro. La bandera no se deforma aunque su proporción no
// coincida con la del hueco, y el resultado conserva el aspecto de la plantilla.
func recortarComoCover(origen image.Image, ancho, alto int) image.Image {
	caja := origen.Bounds()
	proporcion := float64(ancho) / float64(alto)
	origenProporcion := float64(caja.Dx()) / float64(caja.Dy())

	recorte := caja
	if origenProporcion > proporcion {
		// La imagen es más ancha: se recorta a los lados.
		recorteAncho := int(float64(caja.Dy()) * proporcion)
		x := caja.Min.X + (caja.Dx()-recorteAncho)/2
		recorte = image.Rect(x, caja.Min.Y, x+recorteAncho, caja.Max.Y)
	} else {
		// La imagen es más alta: se recorta arriba y abajo.
		recorteAlto := int(float64(caja.Dx()) / proporcion)
		y := caja.Min.Y + (caja.Dy()-recorteAlto)/2
		recorte = image.Rect(caja.Min.X, y, caja.Max.X, y+recorteAlto)
	}

	destino := image.NewRGBA(image.Rect(0, 0, ancho, alto))
	draw.CatmullRom.Scale(destino, destino.Bounds(), origen, recorte, draw.Src, nil)
	return destino
}
