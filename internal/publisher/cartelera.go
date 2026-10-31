package publisher

import (
	"encoding/json"
	"fmt"
	"time"

	"hooty/internal/publisher/cartelera"
)

// CarteleraRenderer envuelve el generador oficial migrado desde Glauk.
type CarteleraRenderer struct{}

// NewCarteleraRenderer construye el renderizador canónico.
func NewCarteleraRenderer() *CarteleraRenderer {
	return &CarteleraRenderer{}
}

// Render calcula Compra/Venta por divisa y dibuja la plantilla oficial con
// constelaciones, banderas y tipografía embebida.
func (r *CarteleraRenderer) Render(c Cartelera) ([]byte, error) {
	monedas := map[string]*cartelera.Moneda{}
	if len(c.Monedas) > 0 {
		if err := json.Unmarshal(c.Monedas, &monedas); err != nil {
			return nil, fmt.Errorf("publisher: decodificar monedas del socio: %w", err)
		}
	}
	if len(monedas) == 0 {
		monedas = make(map[string]*cartelera.Moneda, len(c.Tasas))
		for code := range c.Tasas {
			monedas[code] = &cartelera.Moneda{}
		}
	}
	tasas := make(map[string]any, len(c.Tasas))
	for code, value := range c.Tasas {
		tasas[code] = value
	}
	perfil := cartelera.Perfil{
		Nombre:     c.Socio,
		IDGrupo:    c.ChatID,
		MonedaBase: c.MonedaBase,
		Monedas:    monedas,
	}
	actual := cartelera.Lote{IDTasa: c.Lote, Tasas: tasas}
	instante := c.Generada
	if instante.IsZero() {
		instante = time.Now()
	}
	datos := cartelera.Build(perfil, actual, cartelera.Lote{}, instante)
	return cartelera.GenerarCartelera(datos)
}
