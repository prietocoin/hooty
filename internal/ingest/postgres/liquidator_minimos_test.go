package postgres

import (
	"testing"

	"hooty/internal/domain"
)

func TestCompletarMinimosContablesNoEscribeCeros(t *testing.T) {
	t.Parallel()

	c := domain.Comprobante{
		Tasa1: 41, Monto1: 100, ME1: 0,
		Tasa2: 0, Monto2: 0, ME2: 0,
	}
	prev2 := ladoPrevio{tasa: 38, monto: 90, me: 2.5}
	if err := completarMinimosContables(&c, ladoPrevio{}, prev2); err != nil {
		t.Fatalf("completar: %v", err)
	}
	if c.ME1 != c.Monto1 {
		t.Fatalf("me1 = %v, se esperaba %v", c.ME1, c.Monto1)
	}
	if c.Tasa2 != prev2.tasa || c.Monto2 != prev2.monto || c.ME2 != prev2.me {
		t.Fatalf("lado 2 no conservó el snapshot: %+v", c)
	}
}

func TestCompletarMinimosContablesRechazaLadoSinDatos(t *testing.T) {
	t.Parallel()

	c := domain.Comprobante{}
	if err := completarMinimosContables(&c, ladoPrevio{}, ladoPrevio{}); err == nil {
		t.Fatal("se esperaba error por equivalentes en cero")
	}
}
