package postgres

import (
	"strings"
	"testing"
)

func TestRateCheckAllowsZero(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		definition string
		want       bool
	}{
		{"positiva", "CHECK (((tasa_1 > (0)::numeric) AND (tasa_2 > (0)::numeric)))", false},
		{"no negativa", "CHECK (((tasa_1 >= (0)::numeric) AND (tasa_2 >= (0)::numeric)))", true},
		{"vacía", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rateCheckAllowsZero(tc.definition); got != tc.want {
				t.Fatalf("rateCheckAllowsZero(%q) = %v, se esperaba %v", tc.definition, got, tc.want)
			}
		})
	}
}

func TestRateCheckSQLRelajaLaRestriccion(t *testing.T) {
	t.Parallel()

	if !strings.Contains(sqlRateCheckDefinition, "pg_constraint") ||
		!strings.Contains(sqlRateCheckDefinition, "chk_tasas_positivas") {
		t.Fatalf("la consulta debe leer pg_constraint: %s", sqlRateCheckDefinition)
	}
	if !strings.Contains(sqlDropRateCheck, "DROP CONSTRAINT IF EXISTS chk_tasas_positivas") {
		t.Fatalf("la migración debe eliminar la restricción previa: %s", sqlDropRateCheck)
	}
	for _, fragment := range []string{"comprobantes", "tasa_1 >= 0", "tasa_2 >= 0"} {
		if !strings.Contains(sqlAddRateCheck, fragment) {
			t.Fatalf("la nueva restricción debe contener %q: %s", fragment, sqlAddRateCheck)
		}
	}
}
