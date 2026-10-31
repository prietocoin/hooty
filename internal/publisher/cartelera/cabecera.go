package cartelera

import "github.com/fogleman/gg"

// Medidas del encabezado: el logo manda la altura y el centro de la banda es el
// punto donde se alinean el texto y la insignia.
const (
	altoLogo    = 44.0
	separacion  = 10.0
	centroLinea = margenY + altoLogo/2
)

// pintarCabecera dibuja el isotipo del búho, el nombre del socio y la insignia
// OFICIAL. Se eliminó el prefijo FUNDABLOCK para dejar el nombre como protagonista.
func pintarCabecera(lienzo *gg.Context, datos Payload) error {
	x := pintarLogo(lienzo)

	if err := fuente(lienzo, 30); err != nil {
		return err
	}
	escribir(lienzo, datos.NombreSocio, blanco, x, centroLinea)
	return pintarOficial(lienzo)
}

// pintarOficial dibuja la píldora que marca la cartelera como oficial. El ancho
// incluye los 16 px de relleno de cada lado y el espaciado de las siete letras;
// el texto se apoya a 16 px del borde, que es donde lo dejaría el centrado.
func pintarOficial(lienzo *gg.Context) error {
	if err := fuente(lienzo, 11); err != nil {
		return err
	}
	anchoPildora := anchoEspaciado(lienzo, "OFICIAL") + 24
	altoPildora := 11*1.21 + 10
	x := anchoLienzo - margen - anchoPildora
	y := centroLinea - altoPildora/2
	pildora(lienzo, x, y, anchoPildora, altoPildora, 16, tinteOficial, bordeOficial, 1)
	escribirEspaciado(lienzo, "OFICIAL", ambar, x+12, centroLinea)
	return nil
}
