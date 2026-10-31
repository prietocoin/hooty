package cartelera

import (
	"math"
	"strconv"

	"hooty/internal/domain"
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

// textoDeTasa traduce una tasa al texto que se pinta con la regla financiera:
// entero para >=100, dos decimales para >=1 y tres cifras significativas para
// valores menores. Nunca redondea.
func textoDeTasa(valor *float64) string {
	if valor == nil {
		return "null"
	}
	num := *valor
	if num == 0 {
		return "0.00"
	}
	abs := math.Abs(num)
	switch {
	case abs >= 100:
		return strconv.FormatFloat(math.Trunc(num), 'f', 0, 64)
	case abs >= 1:
		return strconv.FormatFloat(domain.Truncar2(num), 'f', 2, 64)
	default:
		decimales := int(2 - math.Floor(math.Log10(abs)))
		return strconv.FormatFloat(domain.TruncarDecimales(num, decimales), 'f', decimales, 64)
	}
}
