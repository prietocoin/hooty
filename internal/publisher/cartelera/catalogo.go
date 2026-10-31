package cartelera

import "strings"

// flagsByCode traduce el código de divisa a la bandera del renderizador. Es un
// dato, no lógica: cuando se sume una divisa se toca este mapa y nada más.
var flagsByCode = map[string]string{
	"COP": "co", "PEN": "pe", "ARS": "ar", "BRL": "br", "MXN": "mx", "EUR": "eu",
	"USD": "us", "USDT": "us", "PYUSD": "us", "PYG": "py", "VES": "ve", "CLP": "cl",
	"DOP": "do", "CRC": "cr", "BOB": "bo", "ECU": "ec", "PAN": "pa", "CAD": "ca",
}

// countryNamesByCode guarda el nombre visible de cada divisa, igual que en el
// origen y con las grafías sin tilde: cambiarlas alteraría carteleras publicadas.
var countryNamesByCode = map[string]string{
	"COP": "Colombia", "PEN": "Peru", "ARS": "Argentina", "BRL": "Brazil", "MXN": "Mexico", "EUR": "Europa",
	"USD": "EEUU-Zelle", "USDT": "Tether", "PYUSD": "PayPal USD", "PYG": "Paraguay", "VES": "Venezuela", "CLP": "Chile",
	"DOP": "Dominicana", "CRC": "Costa Rica", "BOB": "Bolivia", "ECU": "Ecuador", "PAN": "Panama", "CAD": "Canada",
}

// fixedRateCodes son las divisas que valen 1 aunque el lote no las cotice: son
// referencias duras del negocio, no cotizaciones que puedan faltar.
var fixedRateCodes = map[string]bool{
	"USD": true, "USDT": true, "PYUSD": true, "ECU": true, "PAN": true,
}

// fallbackFlag es la bandera de una divisa fuera del catálogo: se prefiere una
// bandera genérica a omitir el dato, porque el renderizador espera una imagen.
const fallbackFlag = "us"

// Flag devuelve el código ISO de la bandera de una divisa ya normalizada.
func Flag(code string) string {
	if flag, ok := flagsByCode[code]; ok {
		return flag
	}
	return fallbackFlag
}

// CountryName devuelve el nombre visible de una divisa ya normalizada. Una divisa
// fuera del catálogo se muestra por su código, como en el origen: eso permite
// publicar pares nuevos sin tocar el código.
func CountryName(code string) string {
	if name, ok := countryNamesByCode[code]; ok {
		return name
	}
	return code
}

// IsFixedRate indica si la divisa vale 1 por definición; espera un código ya
// normalizado y lo usan tanto la tasa base como cada tarjeta.
func IsFixedRate(code string) bool {
	return fixedRateCodes[code]
}

// NormalizeCode uniforma un código de divisa quitando espacios y pasando a
// mayúsculas: el origen escribe los códigos de forma inconsistente y una
// diferencia de mayúsculas basta para perder una cotización.
func NormalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}
