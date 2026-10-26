package dbschema

import (
	"strings"
	"testing"
)

func TestValidateAceptaContratoCompatible(t *testing.T) {
	t.Parallel()

	required := Schema{
		"comprobantes": {
			"hash_largo":      {Kind: Text},
			"monto_recibo":    {Kind: Numeric},
			"fecha_operacion": {Kind: TimestampWall},
			"creado_en":       {Kind: TimestampWall},
		},
	}
	found := map[string]map[string]string{
		"comprobantes": {
			"hash_largo":      "character varying",
			"monto_recibo":    "numeric",
			"fecha_operacion": "timestamp without time zone",
			"creado_en":       "timestamp without time zone",
		},
	}
	if err := Validate(found, required); err != nil {
		t.Fatalf("contrato válido rechazado: %v", err)
	}
}

func TestValidateDetectaColumnaFaltante(t *testing.T) {
	t.Parallel()

	required := Schema{"impactos": {"recibido_en": {Kind: TimestampWall}}}
	err := Validate(map[string]map[string]string{"impactos": {}}, required)
	if err == nil {
		t.Fatal("se esperaba error por columna faltante")
	}
	for _, fragment := range []string{"impactos.recibido_en", "faltan columnas"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("error sin %q: %v", fragment, err)
		}
	}
}

func TestValidateRechazaTimestampConZona(t *testing.T) {
	t.Parallel()

	required := Schema{"comprobantes": {"fecha_operacion": {Kind: TimestampWall}}}
	found := map[string]map[string]string{
		"comprobantes": {"fecha_operacion": "timestamp with time zone"},
	}
	err := Validate(found, required)
	if err == nil {
		t.Fatal("timestamp with time zone debe rechazarse")
	}
	for _, fragment := range []string{
		"comprobantes.fecha_operacion",
		"esperado timestamp without time zone",
		"encontrado timestamp with time zone",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("error sin %q: %v", fragment, err)
		}
	}
}

func TestValidateRechazaTipoNumericoIncompatible(t *testing.T) {
	t.Parallel()

	required := Schema{"comprobantes": {"monto_recibo": {Kind: Numeric}}}
	found := map[string]map[string]string{
		"comprobantes": {"monto_recibo": "text"},
	}
	err := Validate(found, required)
	if err == nil || !strings.Contains(err.Error(), "monto_recibo") {
		t.Fatalf("se esperaba error de tipo para monto_recibo: %v", err)
	}
}

func TestCompatibleTimestamps(t *testing.T) {
	t.Parallel()

	if !Compatible(TimestampWall, "timestamp without time zone") {
		t.Fatal("timestamp sin zona debe ser compatible")
	}
	if Compatible(TimestampWall, "timestamp with time zone") {
		t.Fatal("timestamp con zona no debe ser compatible")
	}
	if Compatible(TimestampWall, "date") {
		t.Fatal("date no debe ser compatible con timestamp de pared")
	}
}
