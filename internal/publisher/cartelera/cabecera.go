package cartelera

import "github.com/fogleman/gg"

// Medidas del encabezado: el logo manda la altura y el centro de la banda es el
// punto donde se alinean el texto y la insignia.
const (
	altoLogo    = 65.0
	separacion  = 15.0
	centroLinea = margen + altoLogo/2
)

// pintarCabecera dibuja el logo, el nombre de la casa con el del socio y la
// insignia OFICIAL. Si el logo no se pudiera cargar, el texto ocupa su lugar.
func pintarCabecera(lienzo *gg.Context, datos Payload) error {
	x := pintarLogo(lienzo)

	if err := fuente(lienzo, 36); err != nil {
		return err
	}
	x += escribirEspaciado(lienzo, "FUNDABLOCK", blanco, x, centroLinea) + 12
	x += escribir(lienzo, "|", naranja, x, centroLinea) + 12
	escribir(lienzo, datos.NombreSocio, gris, x, centroLinea)
	return pintarOficial(lienzo)
}

// pintarOficial dibuja la píldora que marca la cartelera como oficial. El ancho
// incluye los 16 px de relleno de cada lado y el espaciado de las siete letras;
// el texto se apoya a 16 px del borde, que es donde lo dejaría el centrado.
func pintarOficial(lienzo *gg.Context) error {
	if err := fuente(lienzo, 14); err != nil {
		return err
	}
	anchoPildora := anchoEspaciado(lienzo, "OFICIAL") + 32
	altoPildora := 14*1.21 + 12
	x := anchoLienzo - margen - anchoPildora
	y := centroLinea - altoPildora/2
	pildora(lienzo, x, y, anchoPildora, altoPildora, 20, tinteOficial, bordeOficial, 1)
	escribirEspaciado(lienzo, "OFICIAL", ambar, x+16, centroLinea)
	return nil
}
