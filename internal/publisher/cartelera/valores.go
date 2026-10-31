package cartelera

import (
	"math"
	"strconv"
)

// orElse devuelve el primer valor no vacío, o cadena vacía si todos lo están.
//
// Es el equivalente del operador `||` con el que el servicio original completaba
// los campos del payload: allí `t.bandera || t.code?.toLowerCase()` sustituía una
// bandera ausente por el código en minúsculas, y esa regla debe sobrevivir al
// puerto porque la imagen depende de ella.
func orElse(valores ...string) string {
	for _, v := range valores {
		if v != "" {
			return v
		}
	}
	return ""
}

// tasa traduce un número del cálculo al payload.
//
// El puntero no es un capricho: el cálculo reproduce un defecto del original —una
// tasa del orden de 1e-9 produce NaN, ver calculator/precision.go— y JSON no admite
// ese valor. El servicio Node emitía null porque JSON.stringify lo convierte; Go en
// cambio fallaría al serializar. Convirtiéndolo aquí, la respuesta de /calcular y
// el dibujo dicen lo que decía el original.
func tasa(valor float64) *float64 {
	if math.IsNaN(valor) || math.IsInf(valor, 0) {
		return nil
	}
	return &valor
}

// textoDeTasa traduce una tasa al texto que se pinta: siempre dos decimales
// truncados, sin quitar ceros ni redondear. Un nil se muestra como "null".
func textoDeTasa(valor *float64) string {
	if valor == nil {
		return "null"
	}
	return strconv.FormatFloat(*valor, 'f', 2, 64)
}
