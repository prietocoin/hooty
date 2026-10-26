package api

import (
	"strings"
	"testing"

	"hooty/internal/dbschema"
)

func TestRequiredViewSchemaValidaTipos(t *testing.T) {
	t.Parallel()

	found := map[string]map[string]string{
		"perfil": {
			"nombre":        "text",
			"id_grupo":      "text",
			"rol":           "text",
			"moneda_base":   "text",
			"saldo_inicial": "numeric",
			"mostrar":       "jsonb",
			"monedas":       "jsonb",
			"herencia":      "boolean",
		},
		"tasa": {
			"id_tasa": "text",
			"tasas":   "jsonb",
		},
	}
	if err := dbschema.Validate(found, requiredViewSchema); err != nil {
		t.Fatalf("contrato de vistas válido rechazado: %v", err)
	}

	found["tasa"]["tasas"] = "integer"
	err := dbschema.Validate(found, requiredViewSchema)
	if err == nil || !strings.Contains(err.Error(), "tasa.tasas") {
		t.Fatalf("se esperaba error de tipo en tasa.tasas: %v", err)
	}

	delete(found["perfil"], "monedas")
	if err := dbschema.Validate(found, requiredViewSchema); err == nil ||
		!strings.Contains(err.Error(), "perfil.monedas") {
		t.Fatalf("se esperaba error por columna faltante: %v", err)
	}
}
