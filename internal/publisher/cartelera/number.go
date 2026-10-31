package cartelera

import (
	"strconv"
	"strings"
)

// NormalizeNumber convierte a número los valores heterogéneos que llegan en el
// lote de tasas, donde conviven números y cadenas con separador de miles
// ("4,050.50") o con espacios accidentales.
//
// Un valor no interpretable se trata como cero, igual que en el origen: es
// preferible descartar una divisa antes que publicar una tasa inventada. El
// formato europeo ("3.745,20") no se soporta porque el origen tampoco lo soporta.
func NormalizeNumber(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case string:
		cleaned := strings.TrimSpace(strings.ReplaceAll(typed, ",", ""))
		parsed, err := strconv.ParseFloat(cleaned, 64)
		if err != nil {
			return 0
		}
		return parsed
	}
	return 0
}

// isFalsy reproduce el criterio de "valor vacío" de JavaScript, que el cálculo
// original usa con el operador || para decidir si una tasa ausente vale 1.
//
// La distinción es aritméticamente relevante y está cubierta por los fixtures: el
// número 0 es falso y se sustituye por 1, pero la cadena "0" es verdadera, llega
// hasta la verificación de tasa cero y provoca que la divisa se descarte.
func isFalsy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case bool:
		return !typed
	case float64:
		return typed == 0
	case int:
		return typed == 0
	case string:
		return typed == ""
	}
	return false
}
