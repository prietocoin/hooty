package publisher

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
)

//go:embed assets/Inter-Bold.ttf
var interFont []byte

var (
	fontOnce  sync.Once
	fontFace  *truetype.Font
	fontError error
)

// CarteleraRenderer dibuja una imagen PNG con las tasas del lote.
type CarteleraRenderer struct{}

// NewCarteleraRenderer construye el renderizador canónico.
func NewCarteleraRenderer() *CarteleraRenderer {
	return &CarteleraRenderer{}
}

// Render genera la cartelera del socio.
func (r *CarteleraRenderer) Render(c Cartelera) ([]byte, error) {
	fontOnce.Do(func() {
		fontFace, fontError = truetype.Parse(interFont)
	})
	if fontError != nil {
		return nil, fmt.Errorf("publisher: cargar tipografía: %w", fontError)
	}

	codigos := make([]string, 0, len(c.Tasas))
	for codigo, tasa := range c.Tasas {
		if tasa > 0 {
			codigos = append(codigos, strings.ToUpper(strings.TrimSpace(codigo)))
		}
	}
	sort.Strings(codigos)

	alto := 360 + len(codigos)*72
	dc := gg.NewContext(900, alto)
	dc.SetColor(color.RGBA{R: 9, G: 13, B: 24, A: 255})
	dc.Clear()

	// Encabezado.
	texto(dc, "HOOTY", 48, 44, 68, color.RGBA{R: 34, G: 211, B: 238, A: 255})
	texto(dc, "MERCADO & TASAS", 16, 46, 96, color.RGBA{R: 148, G: 163, B: 184, A: 255})
	texto(dc, "LOTE "+c.Lote, 22, 44, 132, color.RGBA{R: 248, G: 250, B: 252, A: 255})
	texto(dc, strings.ToUpper(c.Socio), 32, 46, 186, color.RGBA{R: 248, G: 250, B: 252, A: 255})

	if c.Prueba {
		dc.SetColor(color.RGBA{R: 245, G: 158, B: 11, A: 255})
		dc.DrawRoundedRectangle(44, 210, 120, 34, 12)
		dc.Fill()
		texto(dc, "PRUEBA", 16, 62, 233, color.RGBA{R: 15, G: 23, B: 42, A: 255})
	}

	// Filas de tasas.
	y := 276.0
	for _, codigo := range codigos {
		dc.SetColor(color.RGBA{R: 21, G: 27, B: 52, A: 255})
		dc.DrawRoundedRectangle(40, y, 820, 56, 14)
		dc.Fill()
		texto(dc, codigo, 24, 64, y+36, color.RGBA{R: 226, G: 232, B: 240, A: 255})
		valor := strconv.FormatFloat(c.Tasas[codigo], 'f', 2, 64)
		textoAnclado(dc, valor, 24, 820, y+36, color.RGBA{R: 52, G: 211, B: 153, A: 255})
		y += 72
	}

	// Pie.
	generada := c.Generada
	if generada.IsZero() {
		generada = time.Now()
	}
	texto(dc, "Generado "+generada.Format("2006-01-02 15:04"), 14, 44, float64(alto-32),
		color.RGBA{R: 100, G: 116, B: 139, A: 255})

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("publisher: codificar PNG: %w", err)
	}
	return buf.Bytes(), nil
}

func texto(dc *gg.Context, valor string, tamano, x, y float64, col color.Color) {
	dc.SetFontFace(truetype.NewFace(fontFace, &truetype.Options{Size: tamano}))
	dc.SetColor(col)
	dc.DrawString(valor, x, y)
}

func textoAnclado(dc *gg.Context, valor string, tamano, x, y float64, col color.Color) {
	dc.SetFontFace(truetype.NewFace(fontFace, &truetype.Options{Size: tamano}))
	dc.SetColor(col)
	dc.DrawStringAnchored(valor, x, y, 1, 0)
}
