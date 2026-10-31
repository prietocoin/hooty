package cartelera

// Estados de tendencia. Son cadenas y no un tipo enumerado porque viajan dentro de
// la cartelera y el renderizador las consume como texto: tiparlas obligaría a
// convertir en cada borde sin ganar ninguna garantía real.
const (
	TrendUp    = "up"
	TrendDown  = "down"
	TrendEqual = "equal"
)

// Trend compara la tasa cruzada de hoy con la del lote anterior.
//
// Se mantiene aparte de la tarjeta porque es la única parte del cálculo que mira
// dos lotes a la vez: cuando aparezca una tendencia más rica (porcentaje de
// variación, flechas dobles) se cambia este archivo y nada más.
//
// No distingue "sin lote anterior" de "valor idéntico": ambos casos producen
// equal, y el llamador decide si tiene con qué comparar antes de invocarla.
//
// Mantenerla pura, sin acceso al lote, es lo que permite probar la regla con dos
// números sueltos en lugar de armar dos lotes completos.
func Trend(current, previous float64) string {
	switch {
	case current > previous:
		return TrendUp
	case current < previous:
		return TrendDown
	}
	return TrendEqual
}
