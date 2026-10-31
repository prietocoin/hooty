package cartelera

// fallbackRate es la tasa que se asigna a una divisa cuando el lote no la trae.
// No es un valor de relleno: es la convención del negocio, que trata estas divisas
// como referencia de valor 1.
const fallbackRate = 1.0

// defaultBaseCurrency es la moneda nativa que se asume cuando el socio no la
// declara.
const defaultBaseCurrency = "USDT"

// BaseRate obtiene la tasa de la moneda nativa del socio, que se usa como
// denominador de toda la triangulación.
//
// Las divisas de referencia valen 1 sin consultar el lote. Para el resto, una
// cotización ausente, cero o no numérica también cae a 1: preferir 1 antes que
// cero evita divisiones imposibles que arruinarían la cartelera completa.
func BaseRate(rates map[string]any, baseCurrency string) float64 {
	code := NormalizeCode(baseCurrency)
	if code == "" {
		code = defaultBaseCurrency
	}
	if IsFixedRate(code) {
		return fallbackRate
	}

	value := NormalizeNumber(rates[code])
	if value > 0 {
		return value
	}
	return fallbackRate
}

// rawOrFallback sustituye por la tasa por defecto cualquier valor que JavaScript
// consideraría vacío, replicando el operador || del origen. Se mantiene separado
// de isFalsy porque ahí vive el criterio y aquí, la política de reemplazo.
func rawOrFallback(raw any) any {
	if isFalsy(raw) {
		return fallbackRate
	}
	return raw
}

// BaseCurrencyName devuelve el nombre de la moneda base tal como se publica.
//
// Deliberadamente NO normaliza: el origen publica la cadena recibida y solo
// recurre al valor por defecto cuando viene vacía. Un socio que declare "pen" ve
// "pen" en su cartelera, aunque internamente la búsqueda use "PEN". El caso está
// fijado en los fixtures.
func BaseCurrencyName(baseCurrency string) string {
	if baseCurrency == "" {
		return defaultBaseCurrency
	}
	return baseCurrency
}
