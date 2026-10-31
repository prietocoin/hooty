// Package calculator reproduce en Go el cálculo de carteleras que hoy ejecuta el
// servicio Node.
//
// Es lógica pura: no lee la base de datos, no llama por red y no renderiza. El
// instante se recibe por parámetro, de modo que cada resultado es reproducible y
// comparable contra la salida original.
package cartelera

import "math"

// Constantes de la regla de precisión. Su valor exacto es parte del contrato: la
// escala redondea a ocho decimales, el épsilon empuja los valores que quedaron a un
// paso del truncado, y los umbrales definen cuántos decimales se publican.
const (
	precisionScale      = 1e8
	precisionEpsilon    = 0.0000001
	integerThreshold    = 99.99
	twoDecimalThreshold = 10
)

// ApplyRatePrecision aplica la regla de precisión de la casa a una tasa.
//
// Redondea primero a ocho decimales y después ajusta la cantidad de decimales al
// orden de magnitud: entero por encima de 99.99, dos decimales a partir de 10 y
// resolución fina en valores pequeños.
//
// Conserva el comportamiento literal del original, incluido su defecto conocido:
// una tasa del orden de 1e-9 se redondea a cero y entra en la rama de magnitud,
// donde el logaritmo de cero produce NaN. Se reproduce a propósito para no alterar
// ninguna cartelera durante la migración; queda anotado como candidato a corregir.
func ApplyRatePrecision(value float64) float64 {
	if math.IsNaN(value) || value == 0 {
		return 0
	}

	sign := 1.0
	if value < 0 {
		sign = -1
	}

	rounded := math.Round(math.Abs(value)*precisionScale) / precisionScale
	return sign * truncateByMagnitude(rounded)
}

// truncateByMagnitude decide cuántos decimales sobreviven según el tamaño de la
// tasa. Se separa del redondeo porque son dos decisiones distintas: una normaliza
// el ruido de coma flotante y la otra define cuántos decimales se publican.
func truncateByMagnitude(value float64) float64 {
	switch {
	case value > integerThreshold:
		return math.Trunc(value)
	case value >= twoDecimalThreshold:
		return math.Trunc((value+precisionEpsilon)*100) / 100
	}

	magnitude := math.Floor(math.Log10(value))
	factor := math.Pow(10, 2-magnitude)
	return math.Trunc((value+precisionEpsilon)*factor) / factor
}
