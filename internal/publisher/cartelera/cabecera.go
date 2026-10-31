package cartelera

import "github.com/fogleman/gg"

// Medidas del encabezado: el logo manda la altura y el centro de la banda es el
// punto donde se alinean el texto y la insignia.
const (
	altoLogo    = 52.0
	separacion  = 10.0
	centroLinea = margen + altoLogo/2
)

// pintarCabecera dibuja el logo, el nombre de la casa con el del socio y la
// insignia OFICIAL. Si el logo no se pudiera cargar, el texto ocupa su lugar.
func pintarCabecera(lienzo *gg.Context, datos Payload) error {
	x := pintarLogo(lienzo)

	if err := fuente(lienzo, 26); err != nil {
		return err
	}
	x += escribirEspaciado(lienzo, "FUNDABLOCK", blanco, x, centroLinea) + 8
	x += escribir(lienzo, "|", naranja, x, centroLinea) + 8
	escribir(lienzo, datos.NombreSocio, gris, x, centroLinea)
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
