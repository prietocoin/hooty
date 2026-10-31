package cartelera

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"sync"

	"golang.org/x/image/draw"
)

// cacheImagen decodifica un recurso embebido una sola vez. El fondo pesa 2 MB:
// decodificarlo en cada petición costaría más que dibujar la cartelera entera.
type cacheImagen struct {
	unaVez  sync.Once
	ruta    string
	imagen  image.Image
	errorAl error
}

// obtener devuelve la imagen ya decodificada, o el error del primer intento.
func (c *cacheImagen) obtener() (image.Image, error) {
	c.unaVez.Do(func() { c.imagen, c.errorAl = decodificar(c.ruta) })
	return c.imagen, c.errorAl
}

var (
	fondo = &cacheImagen{ruta: "assets/background.jpg"}
	logo  = &cacheImagen{ruta: "assets/logo.png"}
	// fondoDifuminado aproxima el `backdrop-blur` de CSS: reduce y vuelve a
	// escalar el fondo una sola vez y luego se dibuja debajo de cada fila.
	fondoDifuminado = &cacheDifuminado{}
)

type cacheDifuminado struct {
	unaVez sync.Once
	imagen image.Image
	error  error
}

func (c *cacheDifuminado) obtener() (image.Image, error) {
	c.unaVez.Do(func() {
		original, err := Fondo()
		if err != nil {
			c.error = err
			return
		}
		caja := original.Bounds()
		pequeno := escalar(original, maxInt(1, caja.Dx()/14), maxInt(1, caja.Dy()/14))
		c.imagen = escalar(pequeno, caja.Dx(), caja.Dy())
	})
	return c.imagen, c.error
}

// FondoDifuminado devuelve la versión suave del fondo para el vidrio de las filas.
func FondoDifuminado() (image.Image, error) { return fondoDifuminado.obtener() }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// decodificar convierte un recurso embebido en imagen. El formato lo resuelven
// los decodificadores importados por efecto: JPEG para el fondo y PNG para el resto.
func decodificar(ruta string) (image.Image, error) {
	datos, err := archivos.ReadFile(ruta)
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", ruta, err)
	}
	imagen, _, err := image.Decode(bytes.NewReader(datos))
	if err != nil {
		return nil, fmt.Errorf("decodificando %s: %w", ruta, err)
	}
	return imagen, nil
}

// Fondo devuelve la imagen de fondo, común a todas las carteleras.
func Fondo() (image.Image, error) { return fondo.obtener() }

// Logo devuelve el logo del encabezado.
func Logo() (image.Image, error) { return logo.obtener() }

// escalar ajusta una imagen al tamaño pedido con interpolación suave. gg dibuja a
// escala 1:1, así que el redimensionado se hace antes de entregársela.
func escalar(origen image.Image, ancho, alto int) image.Image {
	destino := image.NewRGBA(image.Rect(0, 0, ancho, alto))
	draw.CatmullRom.Scale(destino, destino.Bounds(), origen, origen.Bounds(), draw.Src, nil)
	return destino
}
