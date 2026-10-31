package cartelera

import (
	"math"

	"hooty/internal/domain"
)

// ApplyRatePrecision aplica la regla financiera de presentación de tasas:
//   - >= 100: entero truncado,
//   - >= 1 y < 100: dos decimales truncados,
//   - < 1: tres cifras significativas truncadas.
//
// Nunca redondea.
func ApplyRatePrecision(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value == 0 {
		return 0
	}
	abs := math.Abs(value)
	switch {
	case abs >= 100:
		return math.Trunc(value)
	case abs >= 1:
		return domain.Truncar2(value)
	default:
		decimales := int(2 - math.Floor(math.Log10(abs)))
		return domain.TruncarDecimales(value, decimales)
	}
}
