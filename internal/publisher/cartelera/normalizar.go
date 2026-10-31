package cartelera

import (
	"strings"
)

// Normalizar traduce la cartelera del dominio al objeto que devuelve /calcular y
// que alimenta el dibujo, con las filas ya completadas por tarjetaPara.
//
// Se exporta porque los dos usos son el mismo objeto: si el dibujo mirara el
// dominio crudo, la imagen y la API podrían decir cosas distintas sobre una
// bandera ausente o una tasa no finita. Duplicar ese mapeo sería la forma más
// rápida de que divergieran.
//
// Devuelve siempre un slice no nulo para que el JSON sea `[]` y no `null` cuando
// el socio no publica ninguna divisa.
func Normalizar(cartelera Cartelera) Payload {
	filas := make([]Tarjeta, 0, len(cartelera.Tarjetas))
	for _, t := range cartelera.Tarjetas {
		filas = append(filas, tarjetaPara(t))
	}
	return Payload{
		NombreSocio: cartelera.NombreSocio,
		IDGrupo:     cartelera.IDGrupo,
		MonedaBase:  cartelera.MonedaBase,
		LoteTasa:    cartelera.LoteTasa,
		Fecha:       cartelera.Fecha,
		Hora:        cartelera.Hora,
		Tarjetas:    filas,
	}
}

// tarjetaPara completa los tres campos que el origen toleraba vacíos —país, bandera
// y tendencia— y traduce sus dos tasas al tipo que admite JSON.
func tarjetaPara(t TarjetaPais) Tarjeta {
	return Tarjeta{
		Code:       t.Code,
		Pais:       orElse(t.NombrePais, t.Code),
		NombrePais: orElse(t.NombrePais, t.Code),
		Bandera:    orElse(t.Bandera, strings.ToLower(t.Code)),
		Compra:     tasa(t.Compra),
		Venta:      tasa(t.Venta),
		Trend:      orElse(t.Trend, "equal"),
	}
}
