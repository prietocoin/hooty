package clock

import (
	"errors"
	"testing"
	"time"
)

func TestCaracasEsUTC4(t *testing.T) {
	t.Parallel()

	loc := Caracas()
	_, offset := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC).In(loc).Zone()
	if offset != int(CaracasOffset/time.Second) {
		t.Fatalf("offset de Caracas = %d, se esperaba %d", offset, int(CaracasOffset/time.Second))
	}
	if name := loc.String(); name != "America/Caracas" && name != "VET" {
		t.Fatalf("zona inesperada: %q", name)
	}
}

func TestResolveCaracasUsaFallback(t *testing.T) {
	t.Parallel()

	loc := resolveCaracas(func(string) (*time.Location, error) {
		return nil, errors.New("tzdata no disponible")
	})
	_, offset := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).In(loc).Zone()
	if offset != int(CaracasOffset/time.Second) {
		t.Fatalf("fallback debe ser UTC-4, obtuvo %d", offset)
	}
	if loc.String() != "VET" {
		t.Fatalf("nombre del fallback = %q, se esperaba VET", loc.String())
	}
}

func TestFromUnixSegundosYMilisegundos(t *testing.T) {
	t.Parallel()

	const seconds = int64(1_700_000_000)
	fromSeconds := FromUnix(seconds)
	fromMillis := FromUnix(seconds * 1000)
	if !fromSeconds.Equal(fromMillis) {
		t.Fatalf("segundos y milisegundos no coinciden: %v vs %v", fromSeconds, fromMillis)
	}
	if _, offset := fromSeconds.Zone(); offset != int(CaracasOffset/time.Second) {
		t.Fatalf("FromUnix debe devolver hora de Caracas, offset %d", offset)
	}
	if got := fromSeconds.Format("2006-01-02 15:04:05"); got != "2023-11-14 18:13:20" {
		t.Fatalf("hora de Caracas = %q", got)
	}
	if !FromUnix(0).IsZero() {
		t.Fatal("FromUnix(0) debe ser cero")
	}
}

func TestWallClockNoDesplazaLaHora(t *testing.T) {
	t.Parallel()

	utc := time.Date(2026, time.October, 25, 15, 30, 0, 0, time.UTC)
	local := WallClock(utc)
	if local.Hour() != 15 || local.Minute() != 30 {
		t.Fatalf("WallClock desplazó la hora: %v", local)
	}
	if local.Location() != Caracas() {
		t.Fatalf("WallClock debe adjuntar Caracas, obtuvo %v", local.Location())
	}
}

func TestForTimestampConservaHoraDePared(t *testing.T) {
	t.Parallel()

	caracas := time.Date(2026, time.October, 25, 11, 15, 0, 0, Caracas())
	stamp := ForTimestamp(caracas)
	if stamp.Location() != time.UTC {
		t.Fatalf("ForTimestamp debe expresar en UTC, obtuvo %v", stamp.Location())
	}
	if stamp.Hour() != 11 || stamp.Minute() != 15 {
		t.Fatalf("ForTimestamp movió la hora de pared: %v", stamp)
	}
	if !stamp.Equal(time.Date(2026, time.October, 25, 11, 15, 0, 0, time.UTC)) {
		t.Fatalf("instante inesperado: %v", stamp)
	}
}

func TestTodayEsMedianocheDeCaracas(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.October, 25, 23, 59, 0, 0, Caracas())
	today := Today(now)
	if today.Hour() != 0 || today.Minute() != 0 || today.Second() != 0 {
		t.Fatalf("Today no devolvió medianoche: %v", today)
	}
	if today.Location() != Caracas() {
		t.Fatalf("Today perdió la zona: %v", today.Location())
	}
}
