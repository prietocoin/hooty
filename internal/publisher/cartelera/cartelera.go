package cartelera

import (
	"sort"
	"time"
)

// defaultLoteID es el identificador que se publica cuando el lote no declara uno.
const defaultLoteID = "T000"

// Build calcula la cartelera completa de un socio a partir del lote vigente y del
// anterior, con el instante recibido por parámetro para que el cálculo sea
// reproducible. Las tarjetas salen ordenadas por código de divisa: el origen las
// recorría en el orden del archivo JSON, que es un detalle de almacenamiento.
func Build(perfil Perfil, current, previous Lote, instant time.Time) Cartelera {
	baseNow := BaseRate(current.Tasas, perfil.MonedaBase)
	baseBefore := BaseRate(previous.Tasas, perfil.MonedaBase)

	codes := sortedCodes(perfil.Monedas)
	cards := make([]TarjetaPais, 0, len(codes))
	for _, code := range codes {
		if card, ok := cardFor(code, perfil.Monedas[code], current, previous, baseNow, baseBefore); ok {
			cards = append(cards, card)
		}
	}

	date, clock := CaracasStamp(instant)
	return Cartelera{
		NombreSocio: perfil.Nombre,
		IDGrupo:     perfil.IDGrupo,
		MonedaBase:  BaseCurrencyName(perfil.MonedaBase),
		LoteTasa:    loteID(current),
		Fecha:       date,
		Hora:        clock,
		Tarjetas:    cards,
	}
}

// sortedCodes devuelve las claves del mapa en orden alfabético, para que el
// recorrido no dependa del azar del mapa.
func sortedCodes(monedas map[string]*Moneda) []string {
	codes := make([]string, 0, len(monedas))
	for code := range monedas {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

// loteID devuelve el identificador visible del lote, con un valor por defecto
// cuando no viene: la cartelera siempre muestra una referencia.
func loteID(lote Lote) string {
	if lote.IDTasa == "" {
		return defaultLoteID
	}
	return lote.IDTasa
}
