package cartelera

import (
	"embed"
	"fmt"
	"sync"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

// archivos reúne los recursos del dibujo dentro del binario. Se embeben y no se
// leen del disco para que una imagen se pueda generar en cualquier contenedor, sin
// copiar assets ni depender del directorio de trabajo del proceso.
//
//go:embed assets/background.jpg assets/logo.png assets/flags/*.png assets/fonts/Inter-Bold.ttf
var archivos embed.FS

// fuenteInter guarda la tipografía ya parseada. Parsear 344 KB por imagen sería
// trabajo repetido, y el *truetype.Font es inmutable, así que se comparte entre
// peticiones sin riesgo de carrera.
var (
	unaVezFuente sync.Once
	fuenteInter  *truetype.Font
	errorFuente  error
)

// cargarFuente parsea Inter-Bold una sola vez y recuerda el error. El recurso está
// embebido, de modo que un fallo solo puede venir de un archivo corrupto: aun así
// se conserva el diagnóstico en lugar de entrar en pánico.
func cargarFuente() (*truetype.Font, error) {
	unaVezFuente.Do(func() {
		datos, err := archivos.ReadFile("assets/fonts/Inter-Bold.ttf")
		if err != nil {
			errorFuente = fmt.Errorf("leyendo la fuente: %w", err)
			return
		}
		fuenteInter, errorFuente = truetype.Parse(datos)
		if errorFuente != nil {
			errorFuente = fmt.Errorf("parseando la fuente: %w", errorFuente)
		}
	})
	return fuenteInter, errorFuente
}

// Letra devuelve la tipografía al tamaño pedido en píxeles. gg dibuja a 72 DPI,
// así que un punto equivale a un píxel y las medidas coinciden con la plantilla
// original, que trabajaba en píxeles CSS.
func Letra(tamano float64) (font.Face, error) {
	fuente, err := cargarFuente()
	if err != nil {
		return nil, err
	}
	return truetype.NewFace(fuente, &truetype.Options{Size: tamano, DPI: 72}), nil
}
