package cartelera

import "testing"

func TestApplyRatePrecisionRespetaTresReglas(t *testing.T) {
	t.Parallel()

	casos := map[float64]float64{
		1652.409: 1652,
		1035.84:  1035,
		5.649:    5.64,
		19.999:   19.99,
		0.9438:   0.943,
		0.09438:  0.0943,
	}
	for entrada, esperado := range casos {
		if got := ApplyRatePrecision(entrada); got != esperado {
			t.Fatalf("ApplyRatePrecision(%v) = %v, se esperaba %v", entrada, got, esperado)
		}
	}
}

func TestTextoDeTasaMuestraReglasSinRedondear(t *testing.T) {
	t.Parallel()

	casos := map[float64]string{
		1652.40: "1652",
		5.649:   "5.64",
		0.9438:  "0.943",
		0.09438: "0.0943",
	}
	for entrada, esperado := range casos {
		valor := entrada
		if got := textoDeTasa(&valor); got != esperado {
			t.Fatalf("textoDeTasa(%v) = %q, se esperaba %q", entrada, got, esperado)
		}
	}
}
