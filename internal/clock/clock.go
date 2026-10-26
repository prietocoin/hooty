// Package clock concentra la hora oficial de Venezuela.
//
// El servicio no debe llamar time.Now() directamente. Toda marca de tiempo
// operativa se obtiene de un Clock y se persiste en hora de Caracas. El
// paquete embebe tzdata para que America/Caracas exista incluso en imágenes
// mínimas; si la carga falla, cae a UTC-4 fijo (VET), que es el offset vigente
// de Venezuela desde 2016.
package clock

import (
	"sync"
	"time"

	// Garantiza que LoadLocation("America/Caracas") funcione sin tzdata del SO.
	_ "time/tzdata"
)

// CaracasOffset es el offset oficial de Venezuela (UTC-4) usado como fallback.
const CaracasOffset = -4 * time.Hour

var (
	caracasOnce sync.Once
	caracasLoc  *time.Location
)

// Caracas devuelve la zona oficial, con fallback determinista a VET/UTC-4.
func Caracas() *time.Location {
	caracasOnce.Do(func() {
		caracasLoc = resolveCaracas(time.LoadLocation)
	})
	return caracasLoc
}

// resolveCaracas permite probar el fallback sin depender del sistema operativo.
func resolveCaracas(load func(string) (*time.Location, error)) *time.Location {
	if loc, err := load("America/Caracas"); err == nil && loc != nil {
		return loc
	}
	return time.FixedZone("VET", int(CaracasOffset/time.Second))
}

// Clock abstrae el reloj para que los tests sean deterministas.
type Clock interface {
	Now() time.Time
}

// System es el reloj de producción. Siempre devuelve la hora en Caracas.
type System struct{}

// Now implementa Clock.
func (System) Now() time.Time {
	return time.Now().In(Caracas())
}

// Fixed es un reloj inmutable para tests y simulaciones.
type Fixed struct {
	T time.Time
}

// Now implementa Clock. La hora se expresa en Caracas sin desplazar el instante.
func (f Fixed) Now() time.Time {
	if f.T.IsZero() {
		return time.Time{}
	}
	return f.T.In(Caracas())
}

// FromUnix interpreta segundos, milisegundos, microsegundos o nanosegundos y
// devuelve el instante en hora de Venezuela. WhatsApp suele enviar segundos o
// milisegundos; los otros casos se aceptan para no perder eventos antiguos.
func FromUnix(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	switch {
	case value > 100_000_000_000_000_000: // nanosegundos
		return time.Unix(0, value).In(Caracas())
	case value > 100_000_000_000_000: // microsegundos
		return time.UnixMicro(value).In(Caracas())
	case value > 100_000_000_000: // milisegundos
		return time.UnixMilli(value).In(Caracas())
	default: // segundos
		return time.Unix(value, 0).In(Caracas())
	}
}

// WallClock adjunta Caracas a una hora de pared sin mover sus componentes.
//
// Se usa al leer columnas timestamp without time zone: pgx las entrega como
// UTC con la misma hora de pared, y convertirlas con In(Caracas) las
// desplazaría dos veces.
func WallClock(t time.Time) time.Time {
	return relocalize(t, Caracas())
}

// ForTimestamp fija la hora de pared en UTC antes de escribir una columna
// timestamp without time zone. pgx descarta la zona y conserva los
// componentes; expresarlos en UTC evita ambigüedades entre drivers.
func ForTimestamp(t time.Time) time.Time {
	return relocalize(t, time.UTC)
}

// Today devuelve la medianoche de Caracas del día que contiene t.
func Today(t time.Time) time.Time {
	year, month, day := t.In(Caracas()).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, Caracas())
}

func relocalize(t time.Time, loc *time.Location) time.Time {
	if t.IsZero() {
		return t
	}
	return time.Date(
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond(),
		loc,
	)
}
