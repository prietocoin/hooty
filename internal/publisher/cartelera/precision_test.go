package cartelera

import "testing"

func TestApplyRatePrecisionTruncaSiempreDosDecimales(t *testing.T) {
	t.Parallel()

	casos := map[float64]float64{
		1652.409: 1652.40,
		1587.609: 1587.60,
		1.239:    1.23,
		0.279:    0.27,
		-1.239:   -1.23,
	}
	for entrada, esperado := range casos {
		if got := ApplyRatePrecision(entrada); got != esperado {
			t.Fatalf("ApplyRatePrecision(%v) = %v, se esperaba %v", entrada, got, esperado)
		}
	}
}

func TestTextoDeTasaMuestraSiempreDosDecimales(t *testing.T) {
	t.Parallel()

	valor := 1652.40
	if got := textoDeTasa(&valor); got != "1652.40" {
		t.Fatalf("textoDeTasa = %q", got)
	}
	cero := 0.0
	if got := textoDeTasa(&cero); got != "0.00" {
		t.Fatalf("textoDeTasa(0) = %q", got)
	}
}
