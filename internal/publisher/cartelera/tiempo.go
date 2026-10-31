package cartelera

import (
	"fmt"
	"time"
)

// caracasOffset fija la zona de publicación en UTC-4.
//
// Se usa un desplazamiento fijo en lugar de la base de zonas horarias por dos
// razones: Venezuela no aplica horario de verano desde 2016, y depender de la base
// obligaría a empaquetar tzdata dentro de la imagen final. Si algún día hubiera que
// publicar fechas históricas, este es el único lugar que cambia.
const caracasOffset = -4 * 60 * 60

// shortMonths son las abreviaturas que produce el locale es-VE del servicio
// original: en minúscula, sin punto y con "sept" para septiembre. No coinciden con
// las de Go ni con las del formato ISO, y declararlas como dato es lo que garantiza
// que la fecha publicada no cambie de idioma al migrar.
var shortMonths = [12]string{
	"ene", "feb", "mar", "abr", "may", "jun",
	"jul", "ago", "sept", "oct", "nov", "dic",
}

// CaracasStamp devuelve la fecha y la hora que se imprimen en la cartelera.
//
// El separador de la fecha es un guion y el día va con dos dígitos, tal como los
// produce el locale original ("17-oct", "01-ene"). El instante se recibe por
// parámetro, y no se lee del reloj, para que el cálculo completo sea reproducible.
func CaracasStamp(instant time.Time) (date, clock string) {
	local := instant.In(time.FixedZone("America/Caracas", caracasOffset))
	date = fmt.Sprintf("%02d-%s", local.Day(), shortMonths[local.Month()-1])
	clock = local.Format("15:04")
	return date, clock
}
