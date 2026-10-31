package cartelera

import (
	"bytes"
	"image/png"
	"testing"
)

func TestGenerarCarteleraOficialDevuelvePNG(t *testing.T) {
	t.Parallel()

	salida, err := GenerarCartelera(Cartelera{
		NombreSocio: "Glauk",
		MonedaBase:  "USDT",
		LoteTasa:    "T096",
		Fecha:       "31/10/2026",
		Hora:        "10:30",
		Tarjetas: []TarjetaPais{
			{Code: "ARS", NombrePais: "Argentina", Bandera: "ar", Compra: 1622, Venta: 1663, Trend: "up"},
			{Code: "COP", NombrePais: "Colombia", Bandera: "co", Compra: 3094, Venta: 3150, Trend: "equal"},
		},
	})
	if err != nil {
		t.Fatalf("GenerarCartelera: %v", err)
	}
	imagen, err := png.Decode(bytes.NewReader(salida))
	if err != nil {
		t.Fatalf("el resultado no es un PNG válido: %v", err)
	}
	if ancho := imagen.Bounds().Dx(); ancho != anchoLienzo {
		t.Fatalf("ancho = %d, se esperaba %d", ancho, anchoLienzo)
	}
}
