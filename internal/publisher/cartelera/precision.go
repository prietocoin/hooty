package cartelera

import "hooty/internal/domain"

// ApplyRatePrecision aplica la regla financiera inquebrantable: truncamiento
// estricto a dos decimales, sin redondeo y sin precisión adaptativa.
func ApplyRatePrecision(value float64) float64 {
	return domain.Truncar2(value)
}
