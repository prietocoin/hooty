package domain

import (
	"regexp"
	"strings"
)

// socioAbonoCondicionado es el único socio alcanzado por la excepción.
const socioAbonoCondicionado = "IRIS"

// divisaAbonoCondicionada es la única divisa alcanzada por la excepción.
const divisaAbonoCondicionada = "COP"

// captionAbonoIris exige que el caption del comprobante sea únicamente A o a,
// con espacios exteriores tolerados. "Abono", "A 50" o un caption vacío no
// califican como abono.
var captionAbonoIris = regexp.MustCompile(`^\s*[Aa]\s*$`)

// EsIrisCOP indica si el socio y la divisa están alcanzados por la excepción.
func EsIrisCOP(socio, divisa string) bool {
	return strings.EqualFold(strings.TrimSpace(socio), socioAbonoCondicionado) &&
		strings.EqualFold(strings.TrimSpace(divisa), divisaAbonoCondicionada)
}

// EsCaptionAbonoIris valida el único caption que habilita el abono de IRIS/COP.
func EsCaptionAbonoIris(caption string) bool {
	return captionAbonoIris.MatchString(caption)
}

// CaptionAbonoIrisEfectivo devuelve el caption que debe evaluar el clasificador.
// Si cualquiera de los dos mensajes trae A/a exacto, ese caption manda.
func CaptionAbonoIrisEfectivo(caption1, caption2 string) string {
	if EsCaptionAbonoIris(caption1) {
		return caption1
	}
	if EsCaptionAbonoIris(caption2) {
		return caption2
	}
	return caption1
}
