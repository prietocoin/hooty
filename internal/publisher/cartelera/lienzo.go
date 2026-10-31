// Package render presenta una cartelera: la convierte en el JSON que devuelve
// /calcular y en el PNG que se publica, sin depender de un servicio externo.
//
// Dibuja en memoria con fogleman/gg sobre recursos embebidos (fondo, logo,
// banderas y tipografía), de modo que una imagen no necesita disco ni red. No
// conoce Fiber, ni la base de datos, ni el cálculo: recibe la cartelera resuelta.
package cartelera

import (
	"bytes"
	"fmt"

	"github.com/fogleman/gg"
)

// GenerarCartelera dibuja la cartelera y devuelve sus bytes PNG.
//
// Trabaja sobre la misma vista normalizada que publica /calcular y no sobre el
// dominio crudo: así la imagen y la API no pueden decir cosas distintas sobre una
// bandera ausente, una tendencia vacía o una tasa que el cálculo dejó en NaN.
func GenerarCartelera(cartelera Cartelera) ([]byte, error) {
	datos := Normalizar(cartelera)
	lienzo := gg.NewContext(anchoLienzo, altoDelLienzo(len(datos.Tarjetas)))

	pintarFondo(lienzo)
	if err := pintarCabecera(lienzo, datos); err != nil {
		return nil, err
	}
	if err := pintarTarjetas(lienzo, datos.Tarjetas); err != nil {
		return nil, err
	}
	if err := pintarPie(lienzo, datos); err != nil {
		return nil, err
	}

	var salida bytes.Buffer
	if err := lienzo.EncodePNG(&salida); err != nil {
		return nil, fmt.Errorf("codificando el PNG: %w", err)
	}
	return salida.Bytes(), nil
}
