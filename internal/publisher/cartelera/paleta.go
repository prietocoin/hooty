package cartelera

import "image/color"

// Paleta de la plantilla original. Los colores se declaran una sola vez para que
// un ajuste de diseño no quede como un hex suelto en medio del dibujo.
//
// Se usa NRGBA y no RGBA porque los tonos translúcidos se expresan sin premultiplicar:
// así `A: 217` equivale al 0.85 de CSS y el resultado se compone igual que en Node.
var (
	blanco  = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	gris    = color.NRGBA{R: 148, G: 163, B: 184, A: 255} // #94a3b8
	azul    = color.NRGBA{R: 59, G: 130, B: 246, A: 255}  // #3b82f6
	naranja = color.NRGBA{R: 217, G: 119, B: 6, A: 255}   // #d97706
	ambar   = color.NRGBA{R: 251, G: 191, B: 36, A: 255}  // #fbbf24
	verde   = color.NRGBA{R: 52, G: 211, B: 153, A: 255}  // #34d399

	// El rojo es #f87171, el que fija la especificación de la migración. La
	// plantilla heredada usaba #fb7185 (rose-400), un tono casi indistinguible:
	// se aplica el color pedido y la diferencia con las carteleras antiguas es
	// de dos unidades en el canal azul.
	rojo     = color.NRGBA{R: 248, G: 113, B: 113, A: 255} // #f87171
	textoPie = color.NRGBA{R: 226, G: 232, B: 240, A: 255} // #e2e8f0

	tenue        = color.NRGBA{R: 255, G: 255, B: 255, A: 26} // rgba(255,255,255,0.1)
	separador    = color.NRGBA{R: 255, G: 255, B: 255, A: 38} // rgba(255,255,255,0.15)
	bordeBandera = color.NRGBA{R: 255, G: 255, B: 255, A: 51} // rgba(255,255,255,0.2)

	rellenoTarjeta = color.NRGBA{R: 11, G: 19, B: 38, A: 217}  // rgba(11,19,38,0.85)
	bordeTarjeta   = color.NRGBA{R: 59, G: 130, B: 246, A: 89} // rgba(59,130,246,0.35)

	tinteOficial = color.NRGBA{R: 217, G: 119, B: 6, A: 38}
	bordeOficial = color.NRGBA{R: 217, G: 119, B: 6, A: 102}

	tinteVerde = color.NRGBA{R: 52, G: 211, B: 153, A: 38}
	bordeVerde = color.NRGBA{R: 52, G: 211, B: 153, A: 89}
	tinteRojo  = color.NRGBA{R: 248, G: 113, B: 113, A: 38}
	bordeRojo  = color.NRGBA{R: 248, G: 113, B: 113, A: 89}
	tinteAmbar = color.NRGBA{R: 251, G: 191, B: 36, A: 38}
	bordeAmbar = color.NRGBA{R: 251, G: 191, B: 36, A: 89}

	tintePie     = color.NRGBA{R: 15, G: 23, B: 42, A: 217} // rgba(15,23,42,0.85)
	tintePieAlto = color.NRGBA{R: 15, G: 23, B: 42, A: 230} // rgba(15,23,42,0.9)
	bordePie     = color.NRGBA{R: 255, G: 255, B: 255, A: 38}
	bordePieAzul = color.NRGBA{R: 59, G: 130, B: 246, A: 102}

	// El fondo de emergencia es el mismo azul que la plantilla declaraba bajo la
	// imagen: si el recurso faltara, la cartelera seguiría siendo legible.
	rellenoFondo = color.NRGBA{R: 4, G: 10, B: 23, A: 255} // #040a17
)
